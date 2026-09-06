package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func TestTransitionMissionPersists(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, fixtures := seedDraftMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	pending, err := store.TransitionMission(ctx, org.OrganizationID, fixtures.mission.MissionID, domain.MissionPendingApproval, now)
	if err != nil {
		t.Fatalf("to pending: %v", err)
	}
	approved, err := store.TransitionMission(ctx, org.OrganizationID, pending.MissionID, domain.MissionApproved, now)
	if err != nil {
		t.Fatalf("to approved: %v", err)
	}
	got, err := store.GetMission(ctx, org.OrganizationID, approved.MissionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.MissionApproved {
		t.Fatalf("state = %s", got.State)
	}
}

func TestTransitionMissionRejectsInvalidEdge(t *testing.T) {
	store := openTestStore(t)
	org, fixtures := seedDraftMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := store.TransitionMission(context.Background(), org.OrganizationID, fixtures.mission.MissionID, domain.MissionApproved, now)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v", err)
	}
}

func TestIssueOriginatingMandatePersistence(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, fixtures := seedApprovedMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	issued, err := store.IssueOriginatingMandate(ctx, richParams(fixtures), now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := store.GetMandate(ctx, org.OrganizationID, issued.MandateID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentMandateID != nil || got.DelegationDepth != 2 {
		t.Fatalf("unexpected mandate %+v", got)
	}
}

func TestIssueOriginatingMandateRejectsDraftMission(t *testing.T) {
	store := openTestStore(t)
	_, fixtures := seedDraftMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	_, err := store.IssueOriginatingMandate(context.Background(), richParams(fixtures), now)
	if !errors.Is(err, domain.ErrMissionNotApproved) {
		t.Fatalf("err = %v", err)
	}
}

func TestIssueChildMandateAndChainPersistence(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, fixtures := seedApprovedMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	parent, err := store.IssueOriginatingMandate(ctx, richParams(fixtures), now)
	if err != nil {
		t.Fatal(err)
	}
	childP := richParams(fixtures)
	childP.ParentMandateID = ptr(parent.MandateID)
	childP.DelegationDepth = 1
	child, err := store.IssueChildMandate(ctx, childP, now)
	if err != nil {
		t.Fatalf("child: %v", err)
	}
	chain, err := store.ReconstructDelegationChain(ctx, org.OrganizationID, child.MandateID)
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if len(chain) != 2 || chain[0].MandateID != parent.MandateID || chain[1].MandateID != child.MandateID {
		t.Fatalf("chain = %+v", chain)
	}
	verified, err := store.VerifyDelegationChain(ctx, org.OrganizationID, child.MandateID, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(verified) != 2 {
		t.Fatalf("verified len = %d", len(verified))
	}
}

func TestVerifyDelegationChainRejectsPersistedWidenedChild(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, fixtures := seedApprovedMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	parent, err := store.IssueOriginatingMandate(ctx, richParams(fixtures), now)
	if err != nil {
		t.Fatal(err)
	}
	childP := richParams(fixtures)
	childP.ParentMandateID = ptr(parent.MandateID)
	childP.DelegationDepth = 1
	childP.ExecutionAuthority = json.RawMessage(`[{"action":"draft_report"},{"action":"wire_funds"}]`)
	widened, err := domain.NewMandate(childP)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMandate(ctx, widened); err != nil {
		t.Fatalf("storage constructor persisted widened child: %v", err)
	}
	if _, err := store.ReconstructDelegationChain(ctx, org.OrganizationID, widened.MandateID); err != nil {
		t.Fatalf("structural reconstruct: %v", err)
	}
	_, err = store.VerifyDelegationChain(ctx, org.OrganizationID, widened.MandateID, now)
	if !errors.Is(err, domain.ErrBrokenChain) || !errors.Is(err, domain.ErrAmplification) {
		t.Fatalf("verified reconstruct err = %v, want ErrBrokenChain and ErrAmplification", err)
	}
}

func TestIssueChildRejectsAmplificationPersistence(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, fixtures := seedApprovedMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	parent, err := store.IssueOriginatingMandate(ctx, richParams(fixtures), now)
	if err != nil {
		t.Fatal(err)
	}
	childP := richParams(fixtures)
	childP.ParentMandateID = ptr(parent.MandateID)
	childP.DelegationDepth = 1
	childP.ExecutionAuthority = json.RawMessage(`[{"action":"draft_report"},{"action":"wire_funds"}]`)
	_, err = store.IssueChildMandate(ctx, childP, now)
	if !errors.Is(err, domain.ErrAmplification) {
		t.Fatalf("err = %v", err)
	}
}

func TestReconstructChainMissingLinkPersistence(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, fixtures := seedApprovedMission(t, store)
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	parent, err := store.IssueOriginatingMandate(ctx, richParams(fixtures), now)
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := domain.NewMandate(richParams(fixtures))
	if err != nil {
		t.Fatal(err)
	}
	missing := uuid.Must(uuid.NewV7())
	orphan.ParentMandateID = &missing
	if err := store.CreateMandate(ctx, orphan); err == nil {
		t.Fatal("expected missing parent FK to fail")
	}
	_, err = store.ReconstructDelegationChain(ctx, org.OrganizationID, uuid.Must(uuid.NewV7()))
	if !errors.Is(err, domain.ErrBrokenChain) {
		t.Fatalf("missing actor: %v", err)
	}
	_ = parent
}

func TestCreateSubMissionRejectsOutlivingParent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, fixtures := seedApprovedMission(t, store)
	child := fixtures.mission
	child.MissionID = uuid.Must(uuid.NewV7())
	child.ParentMissionID = ptr(fixtures.mission.MissionID)
	child.Expiry = fixtures.mission.Expiry.Add(time.Hour)
	child.State = domain.MissionDraft
	if err := store.CreateMission(ctx, child); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

type engineFixtures struct {
	principal domain.IdentityBinding
	agent     domain.IdentityBinding
	issuer    domain.IdentityBinding
	source    domain.AuthoritySource
	mission   domain.Mission
}

func seedDraftMission(t *testing.T, store *Store) (domain.Organization, engineFixtures) {
	t.Helper()
	return seedMissionState(t, store, "Draft", domain.MissionDraft)
}

func seedApprovedMission(t *testing.T, store *Store) (domain.Organization, engineFixtures) {
	t.Helper()
	return seedMissionState(t, store, "Approved", domain.MissionApproved)
}

func seedMissionState(t *testing.T, store *Store, name string, state domain.MissionState) (domain.Organization, engineFixtures) {
	t.Helper()
	ctx := context.Background()
	org, err := domain.NewOrganization(domain.OrganizationParams{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	principal := mustBinding(t, org.OrganizationID, domain.KindPrincipal, name+"-p")
	agent := mustBinding(t, org.OrganizationID, domain.KindAgent, name+"-a")
	issuer := mustBinding(t, org.OrganizationID, domain.KindIssuer, name+"-i")
	for _, b := range []domain.IdentityBinding{principal, agent, issuer} {
		if err := store.CreateIdentityBinding(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	source, err := domain.NewAuthoritySource(domain.AuthoritySourceParams{
		OrganizationID:   org.OrganizationID,
		Type:             domain.SourceHumanApproval,
		StewardBindingID: principal.BindingID,
		ExternalRef:      "urn:approval:" + name,
		Summary:          name + " source",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAuthoritySource(ctx, source); err != nil {
		t.Fatal(err)
	}
	nb := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	mission, err := domain.NewMission(domain.MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            name + " purpose",
		IntendedOutcome:    name + " outcome",
		NotBefore:          nb,
		Expiry:             exp,
		State:              state,
		AuthoritySourceID:  source.SourceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMission(ctx, mission); err != nil {
		t.Fatal(err)
	}
	return org, engineFixtures{
		principal: principal,
		agent:     agent,
		issuer:    issuer,
		source:    source,
		mission:   mission,
	}
}

func richParams(f engineFixtures) domain.MandateParams {
	return domain.MandateParams{
		Version:                domain.MandateSchemaVersion,
		OrganizationID:         f.mission.OrganizationID,
		PrincipalBindingID:     f.principal.BindingID,
		AgentBindingID:         f.agent.BindingID,
		MissionID:              f.mission.MissionID,
		AuthoritySourceID:      f.source.SourceID,
		Scope:                  json.RawMessage(`{"class":"reporting"}`),
		CommunicationAuthority: json.RawMessage(`[{"destination":"https://finance.example"}]`),
		ExecutionAuthority:     json.RawMessage(`[{"action":"draft_report"}]`),
		Constraints:            json.RawMessage(`{"geo":"NL"}`),
		Budget:                 json.RawMessage(`{"unit":"EUR","amount":"40.00","remaining":"40.00","categories":["travel"]}`),
		NotBefore:              f.mission.NotBefore,
		Expiry:                 f.mission.Expiry,
		IssuedAt:               time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC),
		DelegationDepth:        2,
		ApprovalRequirements:   json.RawMessage(`{"human":true}`),
		EvidenceRequirements:   json.RawMessage(`["call.api"]`),
		IssuerBindingID:        f.issuer.BindingID,
		State:                  domain.MandateActive,
	}
}
