package domain

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewDecisionRequiresReasonsAndAct(t *testing.T) {
	_, err := NewDecision(DecisionParams{
		OrganizationID: uuid.Must(uuid.NewV7()),
		ActType:        ActExecute,
		Act:            json.RawMessage(`{"action":"draft_report"}`),
		Result:         ResultAllow,
		DecidedAt:      testNow(),
	})
	if !errors.Is(err, ErrRequiredField) {
		t.Fatalf("err = %v", err)
	}
}

func TestNewDecisionRejectsUnknownActType(t *testing.T) {
	_, err := NewDecision(DecisionParams{
		OrganizationID: uuid.Must(uuid.NewV7()),
		ActType:        "invent",
		Act:            json.RawMessage(`{}`),
		Result:         ResultDeny,
		Reasons:        []Reason{{Code: ReasonDeniedDefault, Message: "no"}},
		DecidedAt:      testNow(),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestNewDecisionAllowsUnresolvedDeny(t *testing.T) {
	d, err := NewDecision(DecisionParams{
		OrganizationID: uuid.Must(uuid.NewV7()),
		ActType:        ActExecute,
		Act:            json.RawMessage(`{"action":"draft_report"}`),
		Result:         ResultDeny,
		Reasons:        []Reason{{Code: ReasonAuthnNotAuthz, Message: "no mandate"}},
		DecidedAt:      testNow(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.MandateID != nil || d.EvidenceRecordID != nil {
		t.Fatalf("unresolved deny should leave optional ids unset: %+v", d)
	}
}

func TestMatchCommunicationUnknownFailsClosed(t *testing.T) {
	err := MatchCommunication(json.RawMessage(`[{"destination":"https://finance.example"}]`), "https://other.example")
	if !errors.Is(err, ErrUnknownDestination) {
		t.Fatalf("err = %v", err)
	}
}

func TestMatchCommunicationWildcardFailsClosed(t *testing.T) {
	if err := MatchCommunication(json.RawMessage(`[{"destination":"https://finance.example"}]`), "*"); !errors.Is(err, ErrUnknownDestination) {
		t.Fatalf("requested wildcard: %v", err)
	}
	if err := MatchCommunication(json.RawMessage(`[{"destination":"*"}]`), "https://finance.example"); !errors.Is(err, ErrMalformedAuthority) {
		t.Fatalf("listed wildcard: %v", err)
	}
}

func TestMatchExecutionKnownAction(t *testing.T) {
	if err := MatchExecution(json.RawMessage(`[{"action":"draft_report"}]`), "draft_report"); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalRequiredEmptyObject(t *testing.T) {
	need, err := ApprovalRequired(emptyObject())
	if err != nil || need {
		t.Fatalf("need=%v err=%v", need, err)
	}
}

func TestApprovalRequiredHuman(t *testing.T) {
	need, err := ApprovalRequired(json.RawMessage(`{"human":true}`))
	if err != nil || !need {
		t.Fatalf("need=%v err=%v", need, err)
	}
}

func TestAssertBudgetAllowsSpend(t *testing.T) {
	budget := json.RawMessage(`{"unit":"EUR","amount":"40.00","remaining":"10.00","categories":["travel"]}`)
	if err := AssertBudgetAllowsSpend(budget, json.RawMessage(`{"amount":"5.00","unit":"EUR","category":"travel"}`)); err != nil {
		t.Fatal(err)
	}
	if err := AssertBudgetAllowsSpend(budget, json.RawMessage(`{"amount":"11.00","unit":"EUR"}`)); !errors.Is(err, ErrBudgetInsufficient) {
		t.Fatalf("overspend: %v", err)
	}
	if err := AssertBudgetAllowsSpend(emptyObject(), json.RawMessage(`{"amount":"1","unit":"EUR"}`)); !errors.Is(err, ErrBudgetInsufficient) {
		t.Fatalf("empty budget: %v", err)
	}
}

func TestAssertConstraintsHoldGeo(t *testing.T) {
	now := testNow()
	if err := AssertConstraintsHold(json.RawMessage(`{"geo":"NL"}`), json.RawMessage(`{"action":"draft_report","geo":"NL"}`), now); err != nil {
		t.Fatal(err)
	}
	if err := AssertConstraintsHold(json.RawMessage(`{"geo":"NL"}`), json.RawMessage(`{"action":"draft_report"}`), now); !errors.Is(err, ErrConstraintViolated) {
		t.Fatalf("missing geo: %v", err)
	}
	if err := AssertConstraintsHold(json.RawMessage(`{"geo":"NL"}`), json.RawMessage(`{"action":"draft_report","geo":"DE"}`), now); !errors.Is(err, ErrConstraintViolated) {
		t.Fatalf("wrong geo: %v", err)
	}
}

func TestAssertConstraintsHoldTime(t *testing.T) {
	now := testNow()
	if err := AssertConstraintsHold(json.RawMessage(`{"until":"2026-09-06T11:00:00Z"}`), json.RawMessage(`{"action":"draft_report"}`), now); !errors.Is(err, ErrConstraintViolated) {
		t.Fatalf("until passed: %v", err)
	}
}

func TestValidUntilCannotPrecedeDecidedAt(t *testing.T) {
	until := testNow().Add(-time.Minute)
	_, err := NewDecision(DecisionParams{
		OrganizationID: uuid.Must(uuid.NewV7()),
		ActType:        ActExecute,
		Act:            json.RawMessage(`{"action":"draft_report"}`),
		Result:         ResultAllow,
		Reasons:        []Reason{{Code: ReasonAllowed, Message: "ok"}},
		ValidUntil:     &until,
		DecidedAt:      testNow(),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}
