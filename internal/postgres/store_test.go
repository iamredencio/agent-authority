package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := testDSN(t)
	if dsn == "" {
		t.Skip("PostgreSQL is not available (set TEST_DATABASE_URL or install Docker)")
	}
	ctx := context.Background()
	store, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(store.Close)
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func TestMigrateIsIdempotent(t *testing.T) {
	store := openTestStore(t)
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestPersistenceRoundTripAllAggregates(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	org, err := domain.NewOrganization(domain.OrganizationParams{Name: "Ministry of Finance"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrganization(ctx, org); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	gotOrg, err := store.GetOrganization(ctx, org.OrganizationID)
	if err != nil {
		t.Fatalf("get organization: %v", err)
	}
	assertOrgEq(t, org, gotOrg)

	principal := mustBinding(t, org.OrganizationID, domain.KindPrincipal, "entra://users/ada")
	agent := mustBinding(t, org.OrganizationID, domain.KindAgent, "spiffe://cluster/agent/report")
	issuer := mustBinding(t, org.OrganizationID, domain.KindIssuer, "oidc://authority-plane")
	principal.AttestationMeta = json.RawMessage(`{"issuer":"https://login.microsoftonline.com"}`)
	principal, err = domain.NewIdentityBinding(domain.IdentityBindingParams{
		BindingID:       principal.BindingID,
		OrganizationID:  principal.OrganizationID,
		Kind:            principal.Kind,
		Provider:        principal.Provider,
		Subject:         principal.Subject,
		DisplayName:     "Ada",
		AttestationMeta: principal.AttestationMeta,
		Status:          domain.BindingActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []domain.IdentityBinding{principal, agent, issuer} {
		if err := store.CreateIdentityBinding(ctx, b); err != nil {
			t.Fatalf("create binding %s: %v", b.Kind, err)
		}
		got, err := store.GetIdentityBinding(ctx, org.OrganizationID, b.BindingID)
		if err != nil {
			t.Fatalf("get binding %s: %v", b.Kind, err)
		}
		assertBindingEq(t, b, got)
		if got.Provider == "" || got.Subject == "" {
			t.Fatal("identity binding lost external reference")
		}
	}

	source, err := domain.NewAuthoritySource(domain.AuthoritySourceParams{
		OrganizationID:   org.OrganizationID,
		Type:             domain.SourceHumanApproval,
		StewardBindingID: principal.BindingID,
		ExternalRef:      "https://approvals.example/ticket/42",
		EvidencePointer:  "urn:evidence:ticket-42",
		Summary:          "Human approval for quarterly reporting",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAuthoritySource(ctx, source); err != nil {
		t.Fatalf("create authority source: %v", err)
	}
	gotSource, err := store.GetAuthoritySource(ctx, org.OrganizationID, source.SourceID)
	if err != nil {
		t.Fatalf("get authority source: %v", err)
	}
	if gotSource != source {
		t.Fatalf("authority source round-trip mismatch\n got %#v\nwant %#v", gotSource, source)
	}

	nb := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	parentMission, err := domain.NewMission(domain.MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "Quarterly close",
		IntendedOutcome:    "Signed pack",
		NotBefore:          nb,
		Expiry:             exp,
		State:              domain.MissionApproved,
		AuthoritySourceID:  source.SourceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMission(ctx, parentMission); err != nil {
		t.Fatalf("create parent mission: %v", err)
	}
	mission, err := domain.NewMission(domain.MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "Draft finance narrative",
		IntendedOutcome:    "Narrative accepted",
		ParentMissionID:    ptr(parentMission.MissionID),
		NotBefore:          nb,
		Expiry:             exp,
		State:              domain.MissionApproved,
		AuthoritySourceID:  source.SourceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMission(ctx, mission); err != nil {
		t.Fatalf("create mission: %v", err)
	}
	gotMission, err := store.GetMission(ctx, org.OrganizationID, mission.MissionID)
	if err != nil {
		t.Fatalf("get mission: %v", err)
	}
	assertMissionEq(t, mission, gotMission)

	mandate, err := domain.NewMandate(domain.MandateParams{
		Version:                domain.MandateSchemaVersion,
		OrganizationID:         org.OrganizationID,
		PrincipalBindingID:     principal.BindingID,
		AgentBindingID:         agent.BindingID,
		MissionID:              mission.MissionID,
		AuthoritySourceID:      source.SourceID,
		Scope:                  json.RawMessage(`{"class":"reporting"}`),
		CommunicationAuthority: json.RawMessage(`[{"destination":"https://finance.example"}]`),
		ExecutionAuthority:     json.RawMessage(`[{"action":"draft_report"}]`),
		Constraints:            json.RawMessage(`{"geo":"NL"}`),
		Budget:                 json.RawMessage(`{"unit":"EUR","amount":"100.00","remaining":"100.00","categories":["travel"]}`),
		NotBefore:              nb,
		Expiry:                 exp,
		IssuedAt:               nb,
		DelegationDepth:        3,
		ApprovalRequirements:   json.RawMessage(`{"human":true}`),
		EvidenceRequirements:   json.RawMessage(`["call.api"]`),
		IssuerBindingID:        issuer.BindingID,
		State:                  domain.MandateActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMandate(ctx, mandate); err != nil {
		t.Fatalf("create mandate: %v", err)
	}
	gotMandate, err := store.GetMandate(ctx, org.OrganizationID, mandate.MandateID)
	if err != nil {
		t.Fatalf("get mandate: %v", err)
	}
	assertMandateEq(t, mandate, gotMandate)

	child, err := domain.NewMandate(domain.MandateParams{
		Version:                domain.MandateSchemaVersion,
		OrganizationID:         org.OrganizationID,
		PrincipalBindingID:     principal.BindingID,
		AgentBindingID:         agent.BindingID,
		MissionID:              mission.MissionID,
		AuthoritySourceID:      source.SourceID,
		ParentMandateID:        ptr(mandate.MandateID),
		Scope:                  json.RawMessage(`{"class":"reporting"}`),
		CommunicationAuthority: json.RawMessage(`[{"destination":"https://finance.example"}]`),
		ExecutionAuthority:     json.RawMessage(`[{"action":"draft_report"}]`),
		Constraints:            json.RawMessage(`{}`),
		Budget:                 json.RawMessage(`{}`),
		NotBefore:              nb,
		Expiry:                 exp,
		IssuedAt:               nb,
		DelegationDepth:        3,
		ApprovalRequirements:   json.RawMessage(`{}`),
		EvidenceRequirements:   json.RawMessage(`[]`),
		IssuerBindingID:        issuer.BindingID,
		State:                  domain.MandateActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMandate(ctx, child); err != nil {
		t.Fatalf("create child mandate (storage only): %v", err)
	}
	gotChild, err := store.GetMandate(ctx, org.OrganizationID, child.MandateID)
	if err != nil {
		t.Fatal(err)
	}
	if gotChild.ParentMandateID == nil || *gotChild.ParentMandateID != mandate.MandateID {
		t.Fatal("parent_mandate_id did not persist")
	}
}

func TestPersistenceTenantIsolation(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	_, fixturesA := seedOrg(t, store, "Tenant A")
	orgB, fixturesB := seedOrg(t, store, "Tenant B")

	_, err := store.GetIdentityBinding(ctx, orgB.OrganizationID, fixturesA.principal.BindingID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant binding read: %v", err)
	}
	_, err = store.GetMission(ctx, orgB.OrganizationID, fixturesA.mission.MissionID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant mission read: %v", err)
	}
	_, err = store.GetMandate(ctx, orgB.OrganizationID, fixturesA.mandate.MandateID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant mandate read: %v", err)
	}

	crossSource := fixturesA.source
	crossSource.OrganizationID = orgB.OrganizationID
	crossSource.SourceID = uuid.Must(uuid.NewV7())
	if err := store.CreateAuthoritySource(ctx, crossSource); !errors.Is(err, domain.ErrCrossTenant) {
		t.Fatalf("cross-tenant steward: %v", err)
	}

	crossMission := fixturesA.mission
	crossMission.OrganizationID = orgB.OrganizationID
	crossMission.MissionID = uuid.Must(uuid.NewV7())
	crossMission.PrincipalBindingID = fixturesA.principal.BindingID
	if err := store.CreateMission(ctx, crossMission); !errors.Is(err, domain.ErrCrossTenant) {
		t.Fatalf("cross-tenant mission principal: %v", err)
	}

	crossMandate := fixturesB.mandate
	crossMandate.MandateID = uuid.Must(uuid.NewV7())
	crossMandate.MissionID = fixturesA.mission.MissionID
	if err := store.CreateMandate(ctx, crossMandate); !errors.Is(err, domain.ErrCrossTenant) {
		t.Fatalf("cross-tenant mandate mission: %v", err)
	}

	crossParent := fixturesB.mandate
	crossParent.MandateID = uuid.Must(uuid.NewV7())
	crossParent.ParentMandateID = ptr(fixturesA.mandate.MandateID)
	if err := store.CreateMandate(ctx, crossParent); !errors.Is(err, domain.ErrCrossTenant) {
		t.Fatalf("cross-tenant parent mandate: %v", err)
	}
}

func TestPersistenceRejectsMissingMandateRefs(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	_, fixtures := seedOrg(t, store, "Refs")

	missingMission := fixtures.mandate
	missingMission.MandateID = uuid.Must(uuid.NewV7())
	missingMission.MissionID = uuid.Must(uuid.NewV7())
	if err := store.CreateMandate(ctx, missingMission); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("missing mission: %v", err)
	}

	missingSource := fixtures.mandate
	missingSource.MandateID = uuid.Must(uuid.NewV7())
	missingSource.AuthoritySourceID = uuid.Must(uuid.NewV7())
	if err := store.CreateMandate(ctx, missingSource); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("missing authority source: %v", err)
	}

	wrongKind := fixtures.mandate
	wrongKind.MandateID = uuid.Must(uuid.NewV7())
	wrongKind.PrincipalBindingID = fixtures.agent.BindingID
	if err := store.CreateMandate(ctx, wrongKind); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("agent used as principal: %v", err)
	}
}

func TestPersistenceRejectsNegativeDelegationDepth(t *testing.T) {
	store := openTestStore(t)
	_, fixtures := seedOrg(t, store, "Depth")
	bad := fixtures.mandate
	bad.MandateID = uuid.Must(uuid.NewV7())
	bad.DelegationDepth = -2
	if err := store.CreateMandate(context.Background(), bad); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("negative depth: %v", err)
	}
}

func TestIdentityBindingRemainsExternalAfterRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	org, err := domain.NewOrganization(domain.OrganizationParams{Name: "Bindings"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	b, err := domain.NewIdentityBinding(domain.IdentityBindingParams{
		OrganizationID: org.OrganizationID,
		Kind:           domain.KindAgent,
		Provider:       domain.ProviderWorkload,
		Subject:        "arn:aws:iam::123:role/agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIdentityBinding(ctx, b); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetIdentityBinding(ctx, org.OrganizationID, b.BindingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != domain.ProviderWorkload || got.Subject != "arn:aws:iam::123:role/agent" {
		t.Fatalf("external reference changed: %+v", got)
	}
}

type orgFixtures struct {
	principal domain.IdentityBinding
	agent     domain.IdentityBinding
	issuer    domain.IdentityBinding
	source    domain.AuthoritySource
	mission   domain.Mission
	mandate   domain.Mandate
}

func seedOrg(t *testing.T, store *Store, name string) (domain.Organization, orgFixtures) {
	t.Helper()
	ctx := context.Background()
	org, err := domain.NewOrganization(domain.OrganizationParams{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrganization(ctx, org); err != nil {
		t.Fatal(err)
	}
	principal := mustBinding(t, org.OrganizationID, domain.KindPrincipal, name+"-principal")
	agent := mustBinding(t, org.OrganizationID, domain.KindAgent, name+"-agent")
	issuer := mustBinding(t, org.OrganizationID, domain.KindIssuer, name+"-issuer")
	for _, b := range []domain.IdentityBinding{principal, agent, issuer} {
		if err := store.CreateIdentityBinding(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	source, err := domain.NewAuthoritySource(domain.AuthoritySourceParams{
		OrganizationID:   org.OrganizationID,
		Type:             domain.SourceRole,
		StewardBindingID: principal.BindingID,
		ExternalRef:      "role://" + name,
		Summary:          name + " role",
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
		State:              domain.MissionApproved,
		AuthoritySourceID:  source.SourceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMission(ctx, mission); err != nil {
		t.Fatal(err)
	}
	mandate, err := domain.NewMandate(domain.MandateParams{
		Version:                domain.MandateSchemaVersion,
		OrganizationID:         org.OrganizationID,
		PrincipalBindingID:     principal.BindingID,
		AgentBindingID:         agent.BindingID,
		MissionID:              mission.MissionID,
		AuthoritySourceID:      source.SourceID,
		Scope:                  json.RawMessage(`{}`),
		CommunicationAuthority: json.RawMessage(`[]`),
		ExecutionAuthority:     json.RawMessage(`[]`),
		Constraints:            json.RawMessage(`{}`),
		Budget:                 json.RawMessage(`{}`),
		NotBefore:              nb,
		Expiry:                 exp,
		IssuedAt:               nb,
		DelegationDepth:        1,
		ApprovalRequirements:   json.RawMessage(`{}`),
		EvidenceRequirements:   json.RawMessage(`[]`),
		IssuerBindingID:        issuer.BindingID,
		State:                  domain.MandateActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMandate(ctx, mandate); err != nil {
		t.Fatal(err)
	}
	return org, orgFixtures{
		principal: principal,
		agent:     agent,
		issuer:    issuer,
		source:    source,
		mission:   mission,
		mandate:   mandate,
	}
}

func mustBinding(t *testing.T, orgID uuid.UUID, kind domain.BindingKind, subject string) domain.IdentityBinding {
	t.Helper()
	b, err := domain.NewIdentityBinding(domain.IdentityBindingParams{
		OrganizationID: orgID,
		Kind:           kind,
		Provider:       domain.ProviderCustom,
		Subject:        subject,
		Status:         domain.BindingActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func ptr(id uuid.UUID) *uuid.UUID {
	copied := id
	return &copied
}

func assertOrgEq(t *testing.T, want, got domain.Organization) {
	t.Helper()
	if got.OrganizationID != want.OrganizationID || got.Name != want.Name {
		t.Fatalf("organization mismatch got=%+v want=%+v", got, want)
	}
	if !got.CreatedAt.UTC().Truncate(time.Microsecond).Equal(want.CreatedAt.UTC().Truncate(time.Microsecond)) {
		t.Fatalf("created_at got=%s want=%s", got.CreatedAt, want.CreatedAt)
	}
}

func assertBindingEq(t *testing.T, want, got domain.IdentityBinding) {
	t.Helper()
	if got.BindingID != want.BindingID || got.OrganizationID != want.OrganizationID ||
		got.Kind != want.Kind || got.Provider != want.Provider || got.Subject != want.Subject ||
		got.DisplayName != want.DisplayName || got.Status != want.Status {
		t.Fatalf("binding mismatch\n got %+v\nwant %+v", got, want)
	}
	assertJSONEq(t, want.AttestationMeta, got.AttestationMeta)
}

func assertMissionEq(t *testing.T, want, got domain.Mission) {
	t.Helper()
	if got.MissionID != want.MissionID || got.OrganizationID != want.OrganizationID ||
		got.PrincipalBindingID != want.PrincipalBindingID || got.Purpose != want.Purpose ||
		got.IntendedOutcome != want.IntendedOutcome || got.State != want.State ||
		got.AuthoritySourceID != want.AuthoritySourceID {
		t.Fatalf("mission mismatch\n got %+v\nwant %+v", got, want)
	}
	if !equalOptID(want.ParentMissionID, got.ParentMissionID) {
		t.Fatalf("parent_mission_id got=%v want=%v", got.ParentMissionID, want.ParentMissionID)
	}
	assertTimeEq(t, "not_before", want.NotBefore, got.NotBefore)
	assertTimeEq(t, "expiry", want.Expiry, got.Expiry)
}

func assertMandateEq(t *testing.T, want, got domain.Mandate) {
	t.Helper()
	if got.MandateID != want.MandateID || got.Version != want.Version ||
		got.OrganizationID != want.OrganizationID || got.PrincipalBindingID != want.PrincipalBindingID ||
		got.AgentBindingID != want.AgentBindingID || got.MissionID != want.MissionID ||
		got.AuthoritySourceID != want.AuthoritySourceID || got.IssuerBindingID != want.IssuerBindingID ||
		got.DelegationDepth != want.DelegationDepth || got.State != want.State {
		t.Fatalf("mandate scalar mismatch\n got %+v\nwant %+v", got, want)
	}
	if !equalOptID(want.ParentMandateID, got.ParentMandateID) {
		t.Fatalf("parent_mandate_id got=%v want=%v", got.ParentMandateID, want.ParentMandateID)
	}
	assertJSONEq(t, want.Scope, got.Scope)
	assertJSONEq(t, want.CommunicationAuthority, got.CommunicationAuthority)
	assertJSONEq(t, want.ExecutionAuthority, got.ExecutionAuthority)
	assertJSONEq(t, want.Constraints, got.Constraints)
	assertJSONEq(t, want.Budget, got.Budget)
	assertJSONEq(t, want.ApprovalRequirements, got.ApprovalRequirements)
	assertJSONEq(t, want.EvidenceRequirements, got.EvidenceRequirements)
	assertTimeEq(t, "not_before", want.NotBefore, got.NotBefore)
	assertTimeEq(t, "expiry", want.Expiry, got.Expiry)
	assertTimeEq(t, "issued_at", want.IssuedAt, got.IssuedAt)
}

func equalOptID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func assertTimeEq(t *testing.T, field string, want, got time.Time) {
	t.Helper()
	if !got.UTC().Truncate(time.Microsecond).Equal(want.UTC().Truncate(time.Microsecond)) {
		t.Fatalf("%s got=%s want=%s", field, got, want)
	}
}

func assertJSONEq(t *testing.T, want, got json.RawMessage) {
	t.Helper()
	if len(want) == 0 && len(got) == 0 {
		return
	}
	var w, g any
	if len(want) > 0 {
		if err := json.Unmarshal(want, &w); err != nil {
			t.Fatalf("want json: %v", err)
		}
	}
	if len(got) > 0 {
		if err := json.Unmarshal(got, &g); err != nil {
			t.Fatalf("got json: %v", err)
		}
	}
	if !reflect.DeepEqual(w, g) {
		t.Fatalf("json mismatch got=%s want=%s", got, want)
	}
}
