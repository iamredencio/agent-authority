package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/decision"
	"github.com/iamredencio/agent-authority/internal/domain"
	"github.com/iamredencio/agent-authority/internal/policy"
)

func TestDecisionPersistenceRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, fixtures := seedApprovedMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	issued, err := store.IssueOriginatingMandate(ctx, allowParams(fixtures), now)
	if err != nil {
		t.Fatal(err)
	}

	until := now.Add(30 * time.Second)
	d, err := domain.NewDecision(domain.DecisionParams{
		OrganizationID: org.OrganizationID,
		MandateID:      &issued.MandateID,
		MissionID:      &issued.MissionID,
		ActorBindingID: &issued.AgentBindingID,
		ActType:        domain.ActExecute,
		Act:            json.RawMessage(`{"action":"draft_report"}`),
		Result:         domain.ResultAllow,
		Reasons:        []domain.Reason{{Code: domain.ReasonAllowed, Message: "ok"}},
		ValidUntil:     &until,
		DecidedAt:      now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateDecision(ctx, d); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := store.GetDecision(ctx, org.OrganizationID, d.DecisionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result != domain.ResultAllow || got.EvidenceRecordID != nil {
		t.Fatalf("got %+v", got)
	}
	if got.ValidUntil == nil || !got.ValidUntil.Equal(until) {
		t.Fatalf("valid_until=%v", got.ValidUntil)
	}
}

func TestDecisionAPIPersistsDenyWithoutMandate(t *testing.T) {
	store := openTestStore(t)
	org, fixtures := seedApprovedMission(t, store)
	eng := &decision.Engine{
		Store:  store,
		Policy: policy.Default(),
		Now:    func() time.Time { return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC) },
	}
	srv := httptest.NewServer(decision.Handler(eng))
	t.Cleanup(srv.Close)

	body, _ := json.Marshal(map[string]any{
		"organization_id": org.OrganizationID.String(),
		"act_type":        "execute",
		"act":             map[string]any{"action": "draft_report"},
		"identity":        map[string]any{"binding_id": fixtures.agent.BindingID.String()},
	})
	resp, err := http.Post(srv.URL+"/v1/decisions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var out struct {
		DecisionID string `json:"decision_id"`
		Result     string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != "deny" {
		t.Fatalf("result=%s", out.Result)
	}
	id, err := uuid.Parse(out.DecisionID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetDecision(context.Background(), org.OrganizationID, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.MandateID != nil {
		t.Fatalf("deny without mandate should not store a mandate_id: %v", got.MandateID)
	}
}

func TestIdentityBindingBySubject(t *testing.T) {
	store := openTestStore(t)
	org, fixtures := seedApprovedMission(t, store)
	got, err := store.GetIdentityBindingBySubject(context.Background(), org.OrganizationID, fixtures.agent.Provider, fixtures.agent.Subject)
	if err != nil {
		t.Fatal(err)
	}
	if got.BindingID != fixtures.agent.BindingID {
		t.Fatalf("got %s", got.BindingID)
	}
}

func allowParams(f engineFixtures) domain.MandateParams {
	p := richParams(f)
	p.ApprovalRequirements = json.RawMessage(`{}`)
	p.Constraints = json.RawMessage(`{}`)
	return p
}
