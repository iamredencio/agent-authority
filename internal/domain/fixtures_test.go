package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testOrg(t *testing.T, name string) Organization {
	t.Helper()
	org, err := NewOrganization(OrganizationParams{Name: name})
	if err != nil {
		t.Fatalf("NewOrganization: %v", err)
	}
	return org
}

func testBinding(t *testing.T, orgID uuid.UUID, kind BindingKind, subject string) IdentityBinding {
	t.Helper()
	b, err := NewIdentityBinding(IdentityBindingParams{
		OrganizationID: orgID,
		Kind:           kind,
		Provider:       ProviderEntra,
		Subject:        subject,
		DisplayName:    subject,
		Status:         BindingActive,
	})
	if err != nil {
		t.Fatalf("NewIdentityBinding: %v", err)
	}
	return b
}

func testSource(t *testing.T, orgID, steward uuid.UUID) AuthoritySource {
	t.Helper()
	s, err := NewAuthoritySource(AuthoritySourceParams{
		OrganizationID:   orgID,
		Type:             SourceHumanApproval,
		StewardBindingID: steward,
		ExternalRef:      "urn:approval:fixture",
		Summary:          "fixture authority source",
	})
	if err != nil {
		t.Fatalf("NewAuthoritySource: %v", err)
	}
	return s
}

func testWindow() (time.Time, time.Time) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	return now, now.Add(24 * time.Hour)
}

func testNow() time.Time {
	return time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
}

func testMission(t *testing.T, orgID, principal, source uuid.UUID) Mission {
	t.Helper()
	nb, exp := testWindow()
	m, err := NewMission(MissionParams{
		OrganizationID:     orgID,
		PrincipalBindingID: principal,
		Purpose:            "Prepare a quarterly report",
		IntendedOutcome:    "Draft delivered to finance",
		NotBefore:          nb,
		Expiry:             exp,
		State:              MissionApproved,
		AuthoritySourceID:  source,
	})
	if err != nil {
		t.Fatalf("NewMission: %v", err)
	}
	return m
}

func emptyObject() json.RawMessage {
	return json.RawMessage(`{}`)
}

func emptyArray() json.RawMessage {
	return json.RawMessage(`[]`)
}

func validMandateParams(orgID, principal, agent, issuer, mission, source uuid.UUID) MandateParams {
	nb, exp := testWindow()
	return MandateParams{
		Version:                MandateSchemaVersion,
		OrganizationID:         orgID,
		PrincipalBindingID:     principal,
		AgentBindingID:         agent,
		MissionID:              mission,
		AuthoritySourceID:      source,
		Scope:                  json.RawMessage(`{"class":"reporting"}`),
		CommunicationAuthority: json.RawMessage(`[{"destination":"https://finance.example"}]`),
		ExecutionAuthority:     json.RawMessage(`[{"action":"draft_report"}]`),
		Constraints:            emptyObject(),
		Budget:                 json.RawMessage(`{"unit":"EUR","amount":"0","remaining":"0","categories":[]}`),
		NotBefore:              nb,
		Expiry:                 exp,
		IssuedAt:               nb,
		DelegationDepth:        2,
		ApprovalRequirements:   emptyObject(),
		EvidenceRequirements:   emptyArray(),
		IssuerBindingID:        issuer,
		State:                  MandateActive,
	}
}

func testMandate(t *testing.T, orgID, principal, agent, issuer, mission, source uuid.UUID) Mandate {
	t.Helper()
	m, err := NewMandate(validMandateParams(orgID, principal, agent, issuer, mission, source))
	if err != nil {
		t.Fatalf("NewMandate: %v", err)
	}
	return m
}

func ptrID(id uuid.UUID) *uuid.UUID {
	copied := id
	return &copied
}
