package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
	"github.com/iamredencio/agent-authority/internal/policy"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var fixedNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

type allowPolicy struct{}

func (allowPolicy) Allow(context.Context, any) (bool, error) { return true, nil }

type denyPolicy struct{}

func (denyPolicy) Allow(context.Context, any) (bool, error) {
	return false, domain.ErrPolicyDenied
}

type errorPolicy struct{}

func (errorPolicy) Allow(context.Context, any) (bool, error) {
	return false, domain.ErrPolicyError
}

type memoryStore struct {
	orgs      map[uuid.UUID]domain.Organization
	bindings  map[uuid.UUID]domain.IdentityBinding
	mandates  map[uuid.UUID]domain.Mandate
	missions  map[uuid.UUID]domain.Mission
	decisions map[uuid.UUID]domain.Decision
	chainErr  error
	lookupErr error
}

func newMemory() *memoryStore {
	return &memoryStore{
		orgs:      map[uuid.UUID]domain.Organization{},
		bindings:  map[uuid.UUID]domain.IdentityBinding{},
		mandates:  map[uuid.UUID]domain.Mandate{},
		missions:  map[uuid.UUID]domain.Mission{},
		decisions: map[uuid.UUID]domain.Decision{},
	}
}

func (m *memoryStore) GetOrganization(_ context.Context, organizationID uuid.UUID) (domain.Organization, error) {
	if m.lookupErr != nil {
		return domain.Organization{}, m.lookupErr
	}
	org, ok := m.orgs[organizationID]
	if !ok {
		return domain.Organization{}, domain.ErrNotFound
	}
	return org, nil
}

func (m *memoryStore) GetIdentityBinding(_ context.Context, organizationID, bindingID uuid.UUID) (domain.IdentityBinding, error) {
	b, ok := m.bindings[bindingID]
	if !ok || b.OrganizationID != organizationID {
		return domain.IdentityBinding{}, domain.ErrNotFound
	}
	return b, nil
}

func (m *memoryStore) GetIdentityBindingBySubject(_ context.Context, organizationID uuid.UUID, provider domain.IdentityProvider, subject string) (domain.IdentityBinding, error) {
	var found []domain.IdentityBinding
	for _, b := range m.bindings {
		if b.OrganizationID == organizationID && b.Provider == provider && b.Subject == subject {
			found = append(found, b)
		}
	}
	if len(found) != 1 {
		return domain.IdentityBinding{}, domain.ErrNotFound
	}
	return found[0], nil
}

func (m *memoryStore) GetMandate(_ context.Context, organizationID, mandateID uuid.UUID) (domain.Mandate, error) {
	md, ok := m.mandates[mandateID]
	if !ok || md.OrganizationID != organizationID {
		return domain.Mandate{}, domain.ErrNotFound
	}
	return md, nil
}

func (m *memoryStore) GetMission(_ context.Context, organizationID, missionID uuid.UUID) (domain.Mission, error) {
	ms, ok := m.missions[missionID]
	if !ok || ms.OrganizationID != organizationID {
		return domain.Mission{}, domain.ErrNotFound
	}
	return ms, nil
}

func (m *memoryStore) VerifyDelegationChain(_ context.Context, organizationID, mandateID uuid.UUID, now time.Time) ([]domain.Mandate, error) {
	if m.chainErr != nil {
		return nil, m.chainErr
	}
	md, err := m.GetMandate(context.Background(), organizationID, mandateID)
	if err != nil {
		return nil, domain.ErrBrokenChain
	}
	return []domain.Mandate{md}, nil
}

func (m *memoryStore) CreateDecision(_ context.Context, d domain.Decision) error {
	m.decisions[d.DecisionID] = d
	return nil
}

type fixture struct {
	org     domain.Organization
	agent   domain.IdentityBinding
	other   domain.IdentityBinding
	mission domain.Mission
	mandate domain.Mandate
}

func seed(t *testing.T, store *memoryStore) fixture {
	t.Helper()
	org, err := domain.NewOrganization(domain.OrganizationParams{Name: "Finance", CreatedAt: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := domain.NewIdentityBinding(domain.IdentityBindingParams{
		OrganizationID: org.OrganizationID, Kind: domain.KindPrincipal, Provider: domain.ProviderEntra, Subject: "user-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := domain.NewIdentityBinding(domain.IdentityBindingParams{
		OrganizationID: org.OrganizationID, Kind: domain.KindAgent, Provider: domain.ProviderEntra, Subject: "agent-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := domain.NewIdentityBinding(domain.IdentityBindingParams{
		OrganizationID: org.OrganizationID, Kind: domain.KindAgent, Provider: domain.ProviderEntra, Subject: "agent-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := domain.NewIdentityBinding(domain.IdentityBindingParams{
		OrganizationID: org.OrganizationID, Kind: domain.KindIssuer, Provider: domain.ProviderOIDC, Subject: "issuer-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := domain.NewAuthoritySource(domain.AuthoritySourceParams{
		OrganizationID: org.OrganizationID, Type: domain.SourceHumanApproval, StewardBindingID: principal.BindingID,
		ExternalRef: "urn:approval:1", Summary: "approved",
	})
	if err != nil {
		t.Fatal(err)
	}
	mission, err := domain.NewMission(domain.MissionParams{
		OrganizationID: org.OrganizationID, PrincipalBindingID: principal.BindingID,
		Purpose: "Report", IntendedOutcome: "Draft", NotBefore: fixedNow.Add(-time.Hour),
		Expiry: fixedNow.Add(24 * time.Hour), State: domain.MissionApproved, AuthoritySourceID: source.SourceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	mandate, err := domain.NewMandate(domain.MandateParams{
		Version: domain.MandateSchemaVersion, OrganizationID: org.OrganizationID,
		PrincipalBindingID: principal.BindingID, AgentBindingID: agent.BindingID,
		MissionID: mission.MissionID, AuthoritySourceID: source.SourceID,
		Scope:                  json.RawMessage(`{"class":"reporting"}`),
		CommunicationAuthority: json.RawMessage(`[{"destination":"https://finance.example"}]`),
		ExecutionAuthority:     json.RawMessage(`[{"action":"draft_report"}]`),
		Constraints:            json.RawMessage(`{}`),
		Budget:                 json.RawMessage(`{"unit":"EUR","amount":"40.00","remaining":"40.00","categories":["travel"]}`),
		NotBefore:              mission.NotBefore, Expiry: mission.Expiry, IssuedAt: fixedNow,
		DelegationDepth: 2, ApprovalRequirements: json.RawMessage(`{}`), EvidenceRequirements: json.RawMessage(`[]`),
		IssuerBindingID: issuer.BindingID, State: domain.MandateActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	store.orgs[org.OrganizationID] = org
	store.bindings[principal.BindingID] = principal
	store.bindings[agent.BindingID] = agent
	store.bindings[other.BindingID] = other
	store.bindings[issuer.BindingID] = issuer
	store.missions[mission.MissionID] = mission
	store.mandates[mandate.MandateID] = mandate
	return fixture{org: org, agent: agent, other: other, mission: mission, mandate: mandate}
}

func engineWith(store *memoryStore, pol Policy) *Engine {
	return &Engine{Store: store, Policy: pol, Now: func() time.Time { return fixedNow }, AllowTTL: 30 * time.Second}
}

func TestEvaluateStoresEachActType(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	eng := engineWith(store, allowPolicy{})
	cases := []struct {
		actType domain.ActType
		act     string
	}{
		{domain.ActCommunicate, `{"destination":"https://finance.example"}`},
		{domain.ActExecute, `{"action":"draft_report"}`},
		{domain.ActDelegate, `{"child_depth":1}`},
		{domain.ActSpend, `{"amount":"10.00","unit":"EUR","category":"travel"}`},
	}
	for _, tc := range cases {
		t.Run(string(tc.actType), func(t *testing.T) {
			got, err := eng.Evaluate(context.Background(), Request{
				OrganizationID: fx.org.OrganizationID,
				MandateID:      &fx.mandate.MandateID,
				ActType:        tc.actType,
				Act:            json.RawMessage(tc.act),
				Identity:       IdentityClaim{BindingID: &fx.agent.BindingID},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Result != domain.ResultAllow {
				t.Fatalf("result=%s reasons=%v", got.Result, got.Reasons)
			}
			if _, ok := store.decisions[got.DecisionID]; !ok {
				t.Fatal("decision was not stored")
			}
		})
	}
}

func TestValidAuthWithoutMandateIsDeny(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	got, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID,
		ActType:        domain.ActExecute,
		Act:            json.RawMessage(`{"action":"draft_report"}`),
		Identity:       IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonAuthnNotAuthz {
		t.Fatalf("got %+v", got)
	}
	if _, ok := store.decisions[got.DecisionID]; !ok {
		t.Fatal("deny must be stored")
	}
}

func TestUnknownDestinationAndActionDeny(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	eng := engineWith(store, allowPolicy{})
	comm, err := eng.Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActCommunicate, Act: json.RawMessage(`{"destination":"https://other.example"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if comm.Result != domain.ResultDeny || comm.Reasons[0].Code != domain.ReasonUnknownDestination {
		t.Fatalf("communicate: %+v", comm)
	}
	exec, err := eng.Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"wire_funds"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if exec.Result != domain.ResultDeny || exec.Reasons[0].Code != domain.ReasonUnknownAction {
		t.Fatalf("execute: %+v", exec)
	}
}

func TestPolicyDeniesMandatePermittedAct(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	got, err := engineWith(store, denyPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonPolicyDeny {
		t.Fatalf("got %+v", got)
	}
}

func TestPolicyCannotAllowMandateForbiddenAct(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	got, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"wire_funds"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonUnknownAction {
		t.Fatalf("policy must not grant a forbidden action: %+v", got)
	}
}

func TestOpenTelemetrySpanOnEvaluate(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	eng := engineWith(store, allowPolicy{})
	eng.Tracer = tp.Tracer("test")
	if _, err := eng.Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	}); err != nil {
		t.Fatal(err)
	}
	spans := recorder.Ended()
	if len(spans) == 0 || spans[0].Name() != spanName {
		t.Fatalf("spans=%v", spans)
	}
	foundResult := false
	for _, attr := range spans[0].Attributes() {
		if string(attr.Key) == "decision.result" && attr.Value.AsString() == string(domain.ResultAllow) {
			foundResult = true
		}
	}
	if !foundResult {
		t.Fatal("decision.result attribute missing")
	}
}

func TestFailClosedCases(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	eng := engineWith(store, allowPolicy{})

	t.Run("other_agent", func(t *testing.T) {
		got, err := eng.Evaluate(context.Background(), Request{
			OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.other.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonActorMismatch {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("unknown_mandate", func(t *testing.T) {
		missing := uuid.Must(uuid.NewV7())
		got, err := eng.Evaluate(context.Background(), Request{
			OrganizationID: fx.org.OrganizationID, MandateID: &missing,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonAuthnNotAuthz {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("broken_chain", func(t *testing.T) {
		store.chainErr = domain.ErrBrokenChain
		t.Cleanup(func() { store.chainErr = nil })
		got, err := eng.Evaluate(context.Background(), Request{
			OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonBrokenChain {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("policy_error", func(t *testing.T) {
		got, err := engineWith(store, errorPolicy{}).Evaluate(context.Background(), Request{
			OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonPolicyError {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("expired_mandate", func(t *testing.T) {
		expired := fx.mandate
		expired.Expiry = fixedNow.Add(-time.Minute)
		store.mandates[expired.MandateID] = expired
		t.Cleanup(func() { store.mandates[fx.mandate.MandateID] = fx.mandate })
		got, err := eng.Evaluate(context.Background(), Request{
			OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonMandateNotUsable {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("unapproved_mission", func(t *testing.T) {
		ms := fx.mission
		ms.State = domain.MissionSuspended
		store.missions[ms.MissionID] = ms
		t.Cleanup(func() { store.missions[fx.mission.MissionID] = fx.mission })
		got, err := eng.Evaluate(context.Background(), Request{
			OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("cross_tenant_mandate", func(t *testing.T) {
		otherOrg, err := domain.NewOrganization(domain.OrganizationParams{Name: "Other", CreatedAt: fixedNow})
		if err != nil {
			t.Fatal(err)
		}
		store.orgs[otherOrg.OrganizationID] = otherOrg
		got, err := eng.Evaluate(context.Background(), Request{
			OrganizationID: otherOrg.OrganizationID, MandateID: &fx.mandate.MandateID,
			ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
			Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != domain.ResultDeny {
			t.Fatalf("%+v", got)
		}
	})
}

func TestPendingApproval(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	md := fx.mandate
	md.ApprovalRequirements = json.RawMessage(`{"human":true}`)
	store.mandates[md.MandateID] = md
	got, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultPendingApproval || got.Reasons[0].Code != domain.ReasonApprovalRequired {
		t.Fatalf("%+v", got)
	}
}

func TestHTTPDecisionAPI(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	srv := httptest.NewServer(Handler(engineWith(store, policy.Default())))
	t.Cleanup(srv.Close)

	body := map[string]any{
		"organization_id": fx.org.OrganizationID.String(),
		"mandate_id":      fx.mandate.MandateID.String(),
		"act_type":        "execute",
		"act":             map[string]any{"action": "draft_report"},
		"identity":        map[string]any{"binding_id": fx.agent.BindingID.String()},
	}
	raw, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+"/v1/decisions", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["result"] != "allow" {
		t.Fatalf("out=%v", out)
	}
}

func TestHTTPMalformedJSON(t *testing.T) {
	store := newMemory()
	seed(t, store)
	srv := httptest.NewServer(Handler(engineWith(store, allowPolicy{})))
	t.Cleanup(srv.Close)
	resp, err := http.Post(srv.URL+"/v1/decisions", "application/json", strings.NewReader("{"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestOpenAPIDescribesDecisionContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, need := range []string{
		"/v1/decisions",
		"allow",
		"deny",
		"pending_approval",
		"communicate",
		"execute",
		"delegate",
		"spend",
		"identity",
		"organization_id",
		"not a completed authorization decision",
	} {
		if !strings.Contains(doc, need) {
			t.Fatalf("openapi missing %q", need)
		}
	}
}

func TestEmbeddedRegoPolicyDeny(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	pol, err := policy.New("deny.rego", `
package authority.decision
default allow := false
`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := engineWith(store, pol).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonPolicyDeny {
		t.Fatalf("%+v", got)
	}
}

func TestDisabledIdentityIsDeny(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	agent := fx.agent
	agent.Status = domain.BindingDisabled
	store.bindings[agent.BindingID] = agent
	got, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
		Identity: IdentityClaim{BindingID: &agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonIdentityDisabled {
		t.Fatalf("%+v", got)
	}
}

func TestCannotDelegateAtDepthZero(t *testing.T) {
	store := newMemory()
	fx := seed(t, store)
	md := fx.mandate
	md.DelegationDepth = 0
	store.mandates[md.MandateID] = md
	got, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: fx.org.OrganizationID, MandateID: &fx.mandate.MandateID,
		ActType: domain.ActDelegate, Act: json.RawMessage(`{"child_depth":0}`),
		Identity: IdentityClaim{BindingID: &fx.agent.BindingID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultDeny || got.Reasons[0].Code != domain.ReasonCannotDelegate {
		t.Fatalf("%+v", got)
	}
}

func TestMissingOrganizationEvaluateError(t *testing.T) {
	store := newMemory()
	_, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		ActType: domain.ActExecute, Act: json.RawMessage(`{"action":"draft_report"}`),
	})
	if !errors.Is(err, domain.ErrRequiredField) {
		t.Fatalf("err=%v", err)
	}
	if len(store.decisions) != 0 {
		t.Fatal("missing organization must not produce a stored decision")
	}
}

func TestUnknownOrganizationIsNotADecision(t *testing.T) {
	store := newMemory()
	unknown := uuid.Must(uuid.NewV7())
	got, err := engineWith(store, allowPolicy{}).Evaluate(context.Background(), Request{
		OrganizationID: unknown,
		ActType:        domain.ActExecute,
		Act:            json.RawMessage(`{"action":"draft_report"}`),
	})
	if !errors.Is(err, ErrUnknownOrganization) {
		t.Fatalf("err=%v", err)
	}
	if got.DecisionID != uuid.Nil {
		t.Fatalf("unknown organization returned a decision: %+v", got)
	}
	if len(store.decisions) != 0 {
		t.Fatal("unknown organization must not produce a stored decision")
	}
}

func TestHTTPUnknownOrganization(t *testing.T) {
	store := newMemory()
	seed(t, store)
	srv := httptest.NewServer(Handler(engineWith(store, allowPolicy{})))
	t.Cleanup(srv.Close)
	body, _ := json.Marshal(map[string]any{
		"organization_id": uuid.Must(uuid.NewV7()).String(),
		"act_type":        "execute",
		"act":             map[string]any{"action": "draft_report"},
		"identity":        map[string]any{"binding_id": uuid.Must(uuid.NewV7()).String()},
	})
	resp, err := http.Post(srv.URL+"/v1/decisions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["decision_id"]; ok {
		t.Fatalf("unknown organization must not return a decision: %v", out)
	}
	if out["error"] == nil {
		t.Fatalf("expected error body, got %v", out)
	}
	if len(store.decisions) != 0 {
		t.Fatal("unknown organization must not produce a stored decision")
	}
}
