package decision

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

type httpRequest struct {
	OrganizationID string          `json:"organization_id"`
	MandateID      string          `json:"mandate_id"`
	ActType        string          `json:"act_type"`
	Act            json.RawMessage `json:"act"`
	Identity       httpIdentity    `json:"identity"`
}

type httpIdentity struct {
	BindingID string `json:"binding_id"`
	Provider  string `json:"provider"`
	Subject   string `json:"subject"`
}

type httpDecision struct {
	DecisionID     string          `json:"decision_id"`
	OrganizationID string          `json:"organization_id"`
	MandateID      string          `json:"mandate_id,omitempty"`
	MissionID      string          `json:"mission_id,omitempty"`
	ActorBindingID string          `json:"actor_binding_id,omitempty"`
	ActType        string          `json:"act_type"`
	Act            json.RawMessage `json:"act"`
	Result         string          `json:"result"`
	Reasons        []domain.Reason `json:"reasons"`
	ValidUntil     string          `json:"valid_until,omitempty"`
	DecidedAt      string          `json:"decided_at"`
}

type httpError struct {
	Error string `json:"error"`
}

func Handler(engine *Engine) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/decisions", func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			writeError(w, http.StatusBadRequest, "request body is required")
			return
		}
		var body httpRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "malformed json")
			return
		}

		req, err := parseRequest(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		decision, err := engine.Evaluate(r.Context(), req)
		if err != nil {
			if errors.Is(err, domain.ErrRequiredField) || errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, ErrUnknownOrganization) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if decision.DecisionID != uuid.Nil {
				writeDecision(w, http.StatusOK, decision)
				return
			}
			writeError(w, http.StatusInternalServerError, "decision could not be completed")
			return
		}
		writeDecision(w, http.StatusOK, decision)
	})
	return mux
}

func parseRequest(body httpRequest) (Request, error) {
	orgID, err := parseRequiredUUID("organization_id", body.OrganizationID)
	if err != nil {
		return Request{}, err
	}
	mandateID, err := parseOptionalUUID("mandate_id", body.MandateID)
	if err != nil {
		return Request{}, err
	}
	bindingID, err := parseOptionalUUID("identity.binding_id", body.Identity.BindingID)
	if err != nil {
		return Request{}, err
	}
	return Request{
		OrganizationID: orgID,
		MandateID:      mandateID,
		ActType:        domain.ActType(strings.TrimSpace(body.ActType)),
		Act:            body.Act,
		Identity: IdentityClaim{
			BindingID: bindingID,
			Provider:  domain.IdentityProvider(strings.TrimSpace(body.Identity.Provider)),
			Subject:   strings.TrimSpace(body.Identity.Subject),
		},
	}, nil
}

func parseRequiredUUID(field, value string) (uuid.UUID, error) {
	id, err := parseOptionalUUID(field, value)
	if err != nil {
		return uuid.Nil, err
	}
	if id == nil {
		return uuid.Nil, errors.New(field + " is required")
	}
	return *id, nil
}

func parseOptionalUUID(field, value string) (*uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return nil, errors.New(field + " is not a valid UUID")
	}
	return &id, nil
}

func writeDecision(w http.ResponseWriter, status int, d domain.Decision) {
	out := httpDecision{
		DecisionID:     d.DecisionID.String(),
		OrganizationID: d.OrganizationID.String(),
		ActType:        string(d.ActType),
		Act:            d.Act,
		Result:         string(d.Result),
		Reasons:        d.Reasons,
		DecidedAt:      d.DecidedAt.UTC().Format(time.RFC3339),
	}
	if d.MandateID != nil {
		out.MandateID = d.MandateID.String()
	}
	if d.MissionID != nil {
		out.MissionID = d.MissionID.String()
	}
	if d.ActorBindingID != nil {
		out.ActorBindingID = d.ActorBindingID.String()
	}
	if d.ValidUntil != nil {
		out.ValidUntil = d.ValidUntil.UTC().Format(time.RFC3339)
	}
	writeJSON(w, status, out)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, httpError{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
