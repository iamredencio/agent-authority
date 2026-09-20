package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName     = "agent-authority/decision"
	spanName       = "authority.decision.evaluate"
	defaultTTL     = 30 * time.Second
	defaultActJSON = `{}`
)

// IdentityClaim is an untrusted authentication input. It never authorizes.
type IdentityClaim struct {
	BindingID *uuid.UUID
	Provider  domain.IdentityProvider
	Subject   string
}

// Request is one Decision API evaluation.
type Request struct {
	OrganizationID uuid.UUID
	MandateID      *uuid.UUID
	ActType        domain.ActType
	Act            json.RawMessage
	Identity       IdentityClaim
}

// Repository is the authority-plane store used by the PDP.
type Repository interface {
	GetOrganization(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error)
	GetIdentityBinding(ctx context.Context, organizationID, bindingID uuid.UUID) (domain.IdentityBinding, error)
	GetIdentityBindingBySubject(ctx context.Context, organizationID uuid.UUID, provider domain.IdentityProvider, subject string) (domain.IdentityBinding, error)
	GetMandate(ctx context.Context, organizationID, mandateID uuid.UUID) (domain.Mandate, error)
	GetMission(ctx context.Context, organizationID, missionID uuid.UUID) (domain.Mission, error)
	VerifyDelegationChain(ctx context.Context, organizationID, mandateID uuid.UUID, now time.Time) ([]domain.Mandate, error)
	CreateDecision(ctx context.Context, d domain.Decision) error
}

// Policy evaluates extra restriction after mandate checks. It cannot grant authority.
type Policy interface {
	Allow(ctx context.Context, input any) (bool, error)
}

// Engine is the Phase 3 policy decision point.
type Engine struct {
	Store    Repository
	Policy   Policy
	Now      func() time.Time
	AllowTTL time.Duration
	Tracer   trace.Tracer
}

func (e *Engine) now() time.Time {
	if e != nil && e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}

func (e *Engine) ttl() time.Duration {
	if e != nil && e.AllowTTL > 0 {
		return e.AllowTTL
	}
	return defaultTTL
}

func (e *Engine) tracer() trace.Tracer {
	if e != nil && e.Tracer != nil {
		return e.Tracer
	}
	return otel.Tracer(tracerName)
}

// ErrUnknownOrganization is a request error, not a completed authorization decision.
// P3-1 stores a decision only after the tenant boundary is established.
var ErrUnknownOrganization = errors.New("unknown organization")

// Evaluate produces and persists one decision. Policy never overrides a deny.
func (e *Engine) Evaluate(ctx context.Context, req Request) (domain.Decision, error) {
	ctx, span := e.tracer().Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	span.SetAttributes(
		attribute.String("decision.organization_id", req.OrganizationID.String()),
		attribute.String("decision.act_type", string(req.ActType)),
	)

	if req.OrganizationID == uuid.Nil {
		err := fmt.Errorf("%w: organization_id", domain.ErrRequiredField)
		span.RecordError(err)
		span.SetStatus(codes.Error, "organization_id")
		return domain.Decision{}, err
	}

	if _, err := e.Store.GetOrganization(ctx, req.OrganizationID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			err = fmt.Errorf("%w: organization_id", ErrUnknownOrganization)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "organization")
		return domain.Decision{}, err
	}

	decidedAt := e.now()
	built := e.decide(ctx, req, decidedAt)
	if built.Result == domain.ResultAllow {
		until := decidedAt.Add(e.ttl())
		built.ValidUntil = &until
	}

	decision, err := domain.NewDecision(built)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid decision")
		return domain.Decision{}, err
	}

	if err := e.Store.CreateDecision(ctx, decision); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "persist")
		denied, denyErr := domain.NewDecision(domain.DecisionParams{
			OrganizationID: req.OrganizationID,
			MandateID:      decision.MandateID,
			MissionID:      decision.MissionID,
			ActorBindingID: decision.ActorBindingID,
			ActType:        fallbackActType(req.ActType),
			Act:            fallbackAct(req.Act),
			Result:         domain.ResultDeny,
			Reasons: []domain.Reason{{
				Code:    domain.ReasonPersistenceError,
				Message: "decision could not be persisted",
			}},
			DecidedAt: decidedAt,
		})
		if denyErr != nil {
			return domain.Decision{}, err
		}
		return denied, err
	}

	span.SetAttributes(
		attribute.String("decision.id", decision.DecisionID.String()),
		attribute.String("decision.result", string(decision.Result)),
		attribute.String("decision.reason", decision.Reasons[0].Code),
	)
	if decision.Result == domain.ResultDeny {
		span.SetStatus(codes.Error, decision.Reasons[0].Code)
	}
	return decision, nil
}

func (e *Engine) decide(ctx context.Context, req Request, now time.Time) domain.DecisionParams {
	params := domain.DecisionParams{
		OrganizationID: req.OrganizationID,
		ActType:        fallbackActType(req.ActType),
		Act:            fallbackAct(req.Act),
		Result:         domain.ResultDeny,
		DecidedAt:      now,
	}

	if !validActType(req.ActType) {
		params.Reasons = reasons(domain.ReasonMalformedRequest, "act_type is required")
		return params
	}
	if err := requireAct(req.Act); err != nil {
		params.Reasons = reasons(domain.ReasonMalformedRequest, err.Error())
		return params
	}

	actor, err := e.resolveIdentity(ctx, req.OrganizationID, req.Identity)
	if err != nil {
		params.Reasons = reasons(codeFor(err, domain.ReasonIdentityNotFound), err.Error())
		return params
	}
	params.ActorBindingID = &actor.BindingID
	if actor.Status != domain.BindingActive {
		params.Reasons = reasons(domain.ReasonIdentityDisabled, domain.ErrIdentityDisabled.Error())
		return params
	}

	if req.MandateID == nil {
		params.Reasons = reasons(domain.ReasonAuthnNotAuthz, "authentication is not authorization")
		return params
	}

	mandate, err := e.Store.GetMandate(ctx, req.OrganizationID, *req.MandateID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrCrossTenant) {
			params.Reasons = reasons(domain.ReasonAuthnNotAuthz, "no matching mandate")
			return params
		}
		params.Reasons = reasons(codeFor(err, domain.ReasonRevocationUnavailable), err.Error())
		return params
	}
	params.MandateID = &mandate.MandateID
	params.MissionID = &mandate.MissionID

	if mandate.AgentBindingID != actor.BindingID {
		params.Reasons = reasons(domain.ReasonActorMismatch, domain.ErrActorMismatch.Error())
		return params
	}
	if actor.Kind != domain.KindAgent {
		params.Reasons = reasons(domain.ReasonAuthnNotAuthz, "caller is not the mandated agent")
		return params
	}

	mission, err := e.Store.GetMission(ctx, req.OrganizationID, mandate.MissionID)
	if err != nil {
		params.Reasons = reasons(codeFor(err, domain.ReasonBrokenChain), err.Error())
		return params
	}

	if _, err := e.Store.VerifyDelegationChain(ctx, req.OrganizationID, mandate.MandateID, now); err != nil {
		params.Reasons = reasons(codeFor(err, domain.ReasonBrokenChain), err.Error())
		return params
	}

	if err := domain.AssertMandateUsable(mandate, mission, now); err != nil {
		params.Reasons = reasons(codeFor(err, domain.ReasonMandateNotUsable), err.Error())
		return params
	}

	if err := matchAct(req.ActType, mandate, req.Act, now); err != nil {
		params.Reasons = reasons(codeFor(err, domain.ReasonDeniedDefault), err.Error())
		return params
	}

	pending, err := domain.ApprovalRequired(mandate.ApprovalRequirements)
	if err != nil {
		params.Reasons = reasons(domain.ReasonMalformedRequest, err.Error())
		return params
	}

	if e.Policy == nil {
		params.Reasons = reasons(domain.ReasonPolicyError, "policy engine missing")
		return params
	}
	if _, err := e.Policy.Allow(ctx, policyInput(req, mandate, mission, actor, now)); err != nil {
		params.Reasons = reasons(codeFor(err, domain.ReasonPolicyDeny), err.Error())
		return params
	}

	if pending {
		params.Result = domain.ResultPendingApproval
		params.Reasons = reasons(domain.ReasonApprovalRequired, "mandate requires an approval that has not been granted")
		return params
	}

	params.Result = domain.ResultAllow
	params.Reasons = reasons(domain.ReasonAllowed, "mandate and policy authorize the act")
	return params
}

func (e *Engine) resolveIdentity(ctx context.Context, orgID uuid.UUID, claim IdentityClaim) (domain.IdentityBinding, error) {
	if claim.BindingID != nil {
		b, err := e.Store.GetIdentityBinding(ctx, orgID, *claim.BindingID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrCrossTenant) {
				return domain.IdentityBinding{}, fmt.Errorf("%w", domain.ErrNotFound)
			}
			return domain.IdentityBinding{}, err
		}
		if claim.Provider != "" || strings.TrimSpace(claim.Subject) != "" {
			if claim.Provider != "" && b.Provider != claim.Provider {
				return domain.IdentityBinding{}, fmt.Errorf("%w: identity claim mismatch", domain.ErrInvalidInput)
			}
			if strings.TrimSpace(claim.Subject) != "" && b.Subject != claim.Subject {
				return domain.IdentityBinding{}, fmt.Errorf("%w: identity claim mismatch", domain.ErrInvalidInput)
			}
		}
		return b, nil
	}
	if claim.Provider == "" || strings.TrimSpace(claim.Subject) == "" {
		return domain.IdentityBinding{}, fmt.Errorf("%w: identity claim", domain.ErrRequiredField)
	}
	return e.Store.GetIdentityBindingBySubject(ctx, orgID, claim.Provider, claim.Subject)
}

func matchAct(actType domain.ActType, mandate domain.Mandate, act json.RawMessage, now time.Time) error {
	obj, err := decodeAct(act)
	if err != nil {
		return err
	}
	if err := domain.AssertConstraintsHold(mandate.Constraints, act, now); err != nil {
		return err
	}
	switch actType {
	case domain.ActCommunicate:
		destination, _ := obj["destination"].(string)
		if err := domain.MatchCommunication(mandate.CommunicationAuthority, destination); err != nil {
			return err
		}
		if action, ok := obj["action"].(string); ok && action != "" {
			return domain.MatchExecution(mandate.ExecutionAuthority, action)
		}
		return nil
	case domain.ActExecute, domain.ActControl:
		action, _ := obj["action"].(string)
		if err := domain.MatchExecution(mandate.ExecutionAuthority, action); err != nil {
			return err
		}
		if destination, ok := obj["destination"].(string); ok && destination != "" {
			return domain.MatchCommunication(mandate.CommunicationAuthority, destination)
		}
		return nil
	case domain.ActDelegate:
		if mandate.DelegationDepth <= 0 {
			return fmt.Errorf("%w", domain.ErrCannotDelegate)
		}
		return nil
	case domain.ActSpend:
		return domain.AssertBudgetAllowsSpend(mandate.Budget, act)
	default:
		return fmt.Errorf("%w: act_type", domain.ErrInvalidInput)
	}
}

func policyInput(req Request, mandate domain.Mandate, mission domain.Mission, actor domain.IdentityBinding, now time.Time) map[string]any {
	return map[string]any{
		"organization_id": req.OrganizationID.String(),
		"act_type":        string(req.ActType),
		"act":             jsonObject(req.Act),
		"time":            now.Format(time.RFC3339),
		"identity": map[string]any{
			"binding_id": actor.BindingID.String(),
			"kind":       string(actor.Kind),
			"provider":   string(actor.Provider),
			"subject":    actor.Subject,
			"status":     string(actor.Status),
		},
		"mission": map[string]any{
			"mission_id": mission.MissionID.String(),
			"state":      string(mission.State),
			"purpose":    mission.Purpose,
		},
		"mandate": map[string]any{
			"mandate_id":              mandate.MandateID.String(),
			"version":                 mandate.Version,
			"state":                   string(mandate.State),
			"delegation_depth":        mandate.DelegationDepth,
			"scope":                   jsonObject(mandate.Scope),
			"communication_authority": jsonObject(mandate.CommunicationAuthority),
			"execution_authority":     jsonObject(mandate.ExecutionAuthority),
			"constraints":             jsonObject(mandate.Constraints),
			"budget":                  jsonObject(mandate.Budget),
			"approval_requirements":   jsonObject(mandate.ApprovalRequirements),
			"evidence_requirements":   jsonObject(mandate.EvidenceRequirements),
			"authority_source_id":     mandate.AuthoritySourceID.String(),
		},
	}
}

func jsonObject(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return map[string]any{}
	}
	return v
}

func decodeAct(raw json.RawMessage) (map[string]any, error) {
	if err := requireAct(raw); err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%w: act", domain.ErrInvalidInput)
	}
	return obj, nil
}

func requireAct(raw json.RawMessage) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("%w: act", domain.ErrRequiredField)
	}
	if !json.Valid(raw) {
		return fmt.Errorf("%w: act", domain.ErrInvalidInput)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("%w: act must be an object", domain.ErrInvalidInput)
	}
	return nil
}

func validActType(t domain.ActType) bool {
	switch t {
	case domain.ActCommunicate, domain.ActExecute, domain.ActDelegate, domain.ActSpend, domain.ActControl:
		return true
	default:
		return false
	}
}

func fallbackActType(t domain.ActType) domain.ActType {
	if validActType(t) {
		return t
	}
	return domain.ActExecute
}

func fallbackAct(raw json.RawMessage) json.RawMessage {
	if len(strings.TrimSpace(string(raw))) == 0 || !json.Valid(raw) {
		return json.RawMessage(defaultActJSON)
	}
	return raw
}

func reasons(code, message string) []domain.Reason {
	return []domain.Reason{{Code: code, Message: message}}
}

func codeFor(err error, fallback string) string {
	switch {
	case errors.Is(err, domain.ErrUnknownDestination):
		return domain.ReasonUnknownDestination
	case errors.Is(err, domain.ErrUnknownAction):
		return domain.ReasonUnknownAction
	case errors.Is(err, domain.ErrConstraintViolated):
		return domain.ReasonConstraintViolated
	case errors.Is(err, domain.ErrBudgetInsufficient):
		return domain.ReasonBudgetInsufficient
	case errors.Is(err, domain.ErrCannotDelegate):
		return domain.ReasonCannotDelegate
	case errors.Is(err, domain.ErrMissionNotApproved):
		return domain.ReasonMissionNotApproved
	case errors.Is(err, domain.ErrMissionOutOfWindow):
		return domain.ReasonMissionOutOfWindow
	case errors.Is(err, domain.ErrMandateNotUsable):
		return domain.ReasonMandateNotUsable
	case errors.Is(err, domain.ErrBrokenChain):
		return domain.ReasonBrokenChain
	case errors.Is(err, domain.ErrCrossTenant):
		return domain.ReasonCrossTenant
	case errors.Is(err, domain.ErrActorMismatch):
		return domain.ReasonActorMismatch
	case errors.Is(err, domain.ErrIdentityDisabled):
		return domain.ReasonIdentityDisabled
	case errors.Is(err, domain.ErrPolicyDenied):
		return domain.ReasonPolicyDeny
	case errors.Is(err, domain.ErrPolicyError):
		return domain.ReasonPolicyError
	case errors.Is(err, domain.ErrNotFound):
		return fallback
	case errors.Is(err, domain.ErrRequiredField), errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrMalformedAuthority):
		return domain.ReasonMalformedRequest
	default:
		return fallback
	}
}
