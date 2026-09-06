package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewMission(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "principal-1")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	m, err := NewMissionWithRelations(MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "Reconcile invoices",
		IntendedOutcome:    "Exceptions list",
		NotBefore:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Expiry:             time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		State:              MissionDraft,
		AuthoritySourceID:  source.SourceID,
	}, MissionRelations{Principal: principal, Source: source})
	if err != nil {
		t.Fatalf("NewMissionWithRelations: %v", err)
	}
	if m.State != MissionDraft {
		t.Fatalf("state = %q", m.State)
	}
	if m.Purpose == "" || m.IntendedOutcome == "" {
		t.Fatal("purpose and intended_outcome are required")
	}
}

func TestNewMissionMissingFields(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	nb, exp := testWindow()
	base := MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "purpose",
		IntendedOutcome:    "outcome",
		NotBefore:          nb,
		Expiry:             exp,
		State:              MissionApproved,
		AuthoritySourceID:  source.SourceID,
	}
	cases := []struct {
		name string
		mut  func(*MissionParams)
		want error
	}{
		{"missing organization", func(p *MissionParams) { p.OrganizationID = uuid.Nil }, ErrRequiredField},
		{"missing principal", func(p *MissionParams) { p.PrincipalBindingID = uuid.Nil }, ErrRequiredField},
		{"missing purpose", func(p *MissionParams) { p.Purpose = "" }, ErrRequiredField},
		{"missing intended_outcome", func(p *MissionParams) { p.IntendedOutcome = "" }, ErrRequiredField},
		{"missing not_before", func(p *MissionParams) { p.NotBefore = time.Time{} }, ErrRequiredField},
		{"missing expiry", func(p *MissionParams) { p.Expiry = time.Time{} }, ErrRequiredField},
		{"missing source", func(p *MissionParams) { p.AuthoritySourceID = uuid.Nil }, ErrRequiredField},
		{"invalid state", func(p *MissionParams) { p.State = "running" }, ErrInvalidInput},
		{"expiry before not_before", func(p *MissionParams) {
			p.NotBefore = exp
			p.Expiry = nb
		}, ErrInvalidInput},
		{"nil parent id pointer value", func(p *MissionParams) { p.ParentMissionID = ptrID(uuid.Nil) }, ErrInvalidReference},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mut(&p)
			_, err := NewMission(p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMissionRejectsCrossTenantRelations(t *testing.T) {
	orgA := testOrg(t, "A")
	orgB := testOrg(t, "B")
	principalA := testBinding(t, orgA.OrganizationID, KindPrincipal, "p-a")
	principalB := testBinding(t, orgB.OrganizationID, KindPrincipal, "p-b")
	sourceA := testSource(t, orgA.OrganizationID, principalA.BindingID)
	sourceB := testSource(t, orgB.OrganizationID, principalB.BindingID)
	parentB := testMission(t, orgB.OrganizationID, principalB.BindingID, sourceB.SourceID)

	nb, exp := testWindow()
	p := MissionParams{
		OrganizationID:     orgA.OrganizationID,
		PrincipalBindingID: principalB.BindingID,
		Purpose:            "x",
		IntendedOutcome:    "y",
		NotBefore:          nb,
		Expiry:             exp,
		State:              MissionApproved,
		AuthoritySourceID:  sourceA.SourceID,
	}
	_, err := NewMissionWithRelations(p, MissionRelations{Principal: principalB, Source: sourceA})
	if !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("principal cross-tenant err = %v", err)
	}

	p.PrincipalBindingID = principalA.BindingID
	p.AuthoritySourceID = sourceB.SourceID
	_, err = NewMissionWithRelations(p, MissionRelations{Principal: principalA, Source: sourceB})
	if !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("source cross-tenant err = %v", err)
	}

	p.AuthoritySourceID = sourceA.SourceID
	p.ParentMissionID = ptrID(parentB.MissionID)
	_, err = NewMissionWithRelations(p, MissionRelations{Principal: principalA, Source: sourceA, Parent: &parentB})
	if !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("parent mission cross-tenant err = %v", err)
	}
}

func TestMissionRejectsNonPrincipalBinding(t *testing.T) {
	org := testOrg(t, "Org")
	agent := testBinding(t, org.OrganizationID, KindAgent, "agent")
	source := testSource(t, org.OrganizationID, agent.BindingID)
	nb, exp := testWindow()
	_, err := NewMissionWithRelations(MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: agent.BindingID,
		Purpose:            "x",
		IntendedOutcome:    "y",
		NotBefore:          nb,
		Expiry:             exp,
		State:              MissionApproved,
		AuthoritySourceID:  source.SourceID,
	}, MissionRelations{Principal: agent, Source: source})
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("err = %v, want ErrInvalidReference", err)
	}
}
