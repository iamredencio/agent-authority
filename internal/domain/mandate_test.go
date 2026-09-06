package domain

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewMandateRequiredFields(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	mission := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)

	m, err := NewMandateWithRelations(
		validMandateParams(org.OrganizationID, principal.BindingID, agent.BindingID, issuer.BindingID, mission.MissionID, source.SourceID),
		MandateRelations{Principal: principal, Agent: agent, Issuer: issuer, Mission: mission, Source: source},
	)
	if err != nil {
		t.Fatalf("NewMandateWithRelations: %v", err)
	}
	if m.Version != MandateSchemaVersion {
		t.Fatalf("version = %q", m.Version)
	}
	if m.DelegationDepth != 2 {
		t.Fatalf("delegation_depth = %d", m.DelegationDepth)
	}
	if m.ParentMandateID != nil {
		t.Fatal("originating mandate must allow a null parent")
	}
}

func TestNewMandateMissingRequiredFields(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	mission := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)
	base := validMandateParams(org.OrganizationID, principal.BindingID, agent.BindingID, issuer.BindingID, mission.MissionID, source.SourceID)

	cases := []struct {
		name string
		mut  func(*MandateParams)
		want error
	}{
		{"missing organization", func(p *MandateParams) { p.OrganizationID = uuid.Nil }, ErrRequiredField},
		{"missing principal", func(p *MandateParams) { p.PrincipalBindingID = uuid.Nil }, ErrRequiredField},
		{"missing agent", func(p *MandateParams) { p.AgentBindingID = uuid.Nil }, ErrRequiredField},
		{"missing mission", func(p *MandateParams) { p.MissionID = uuid.Nil }, ErrRequiredField},
		{"missing authority source", func(p *MandateParams) { p.AuthoritySourceID = uuid.Nil }, ErrRequiredField},
		{"missing issuer", func(p *MandateParams) { p.IssuerBindingID = uuid.Nil }, ErrRequiredField},
		{"missing scope", func(p *MandateParams) { p.Scope = nil }, ErrRequiredField},
		{"missing communication_authority", func(p *MandateParams) { p.CommunicationAuthority = nil }, ErrRequiredField},
		{"missing execution_authority", func(p *MandateParams) { p.ExecutionAuthority = nil }, ErrRequiredField},
		{"missing constraints", func(p *MandateParams) { p.Constraints = nil }, ErrRequiredField},
		{"missing budget", func(p *MandateParams) { p.Budget = nil }, ErrRequiredField},
		{"missing approval_requirements", func(p *MandateParams) { p.ApprovalRequirements = nil }, ErrRequiredField},
		{"missing evidence_requirements", func(p *MandateParams) { p.EvidenceRequirements = nil }, ErrRequiredField},
		{"json null scope", func(p *MandateParams) { p.Scope = json.RawMessage(`null`) }, ErrRequiredField},
		{"invalid communication json", func(p *MandateParams) { p.CommunicationAuthority = json.RawMessage(`{`) }, ErrInvalidInput},
		{"missing not_before", func(p *MandateParams) { p.NotBefore = time.Time{} }, ErrRequiredField},
		{"missing expiry", func(p *MandateParams) { p.Expiry = time.Time{} }, ErrRequiredField},
		{"missing issued_at", func(p *MandateParams) { p.IssuedAt = time.Time{} }, ErrRequiredField},
		{"invalid state", func(p *MandateParams) { p.State = "issued" }, ErrInvalidInput},
		{"nil parent id value", func(p *MandateParams) { p.ParentMandateID = ptrID(uuid.Nil) }, ErrInvalidReference},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mut(&p)
			_, err := NewMandate(p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewMandateRejectsNegativeDelegationDepth(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	mission := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)
	p := validMandateParams(org.OrganizationID, principal.BindingID, agent.BindingID, issuer.BindingID, mission.MissionID, source.SourceID)
	p.DelegationDepth = -1
	_, err := NewMandate(p)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestNewMandateAllowsZeroDelegationDepth(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	mission := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)
	p := validMandateParams(org.OrganizationID, principal.BindingID, agent.BindingID, issuer.BindingID, mission.MissionID, source.SourceID)
	p.DelegationDepth = 0
	m, err := NewMandate(p)
	if err != nil {
		t.Fatalf("zero delegation_depth should be allowed: %v", err)
	}
	if m.DelegationDepth != 0 {
		t.Fatalf("delegation_depth = %d", m.DelegationDepth)
	}
}

func TestMandateStoresParentWithoutDelegationEngine(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	mission := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)
	parent := testMandate(t, org.OrganizationID, principal.BindingID, agent.BindingID, issuer.BindingID, mission.MissionID, source.SourceID)

	p := validMandateParams(org.OrganizationID, principal.BindingID, agent.BindingID, issuer.BindingID, mission.MissionID, source.SourceID)
	p.ParentMandateID = ptrID(parent.MandateID)
	p.DelegationDepth = parent.DelegationDepth
	child, err := NewMandateWithRelations(p, MandateRelations{
		Principal: principal, Agent: agent, Issuer: issuer, Mission: mission, Source: source, Parent: &parent,
	})
	if err != nil {
		t.Fatalf("storing parent_mandate_id must not run attenuation: %v", err)
	}
	if child.ParentMandateID == nil || *child.ParentMandateID != parent.MandateID {
		t.Fatal("parent_mandate_id was not stored")
	}
}

func TestMandateRejectsCrossTenantRelations(t *testing.T) {
	orgA := testOrg(t, "A")
	orgB := testOrg(t, "B")
	pA := testBinding(t, orgA.OrganizationID, KindPrincipal, "p-a")
	aA := testBinding(t, orgA.OrganizationID, KindAgent, "a-a")
	iA := testBinding(t, orgA.OrganizationID, KindIssuer, "i-a")
	sA := testSource(t, orgA.OrganizationID, pA.BindingID)
	mA := testMission(t, orgA.OrganizationID, pA.BindingID, sA.SourceID)

	pB := testBinding(t, orgB.OrganizationID, KindPrincipal, "p-b")
	aB := testBinding(t, orgB.OrganizationID, KindAgent, "a-b")
	iB := testBinding(t, orgB.OrganizationID, KindIssuer, "i-b")
	sB := testSource(t, orgB.OrganizationID, pB.BindingID)
	mB := testMission(t, orgB.OrganizationID, pB.BindingID, sB.SourceID)
	parentB := testMandate(t, orgB.OrganizationID, pB.BindingID, aB.BindingID, iB.BindingID, mB.MissionID, sB.SourceID)

	base := validMandateParams(orgA.OrganizationID, pA.BindingID, aA.BindingID, iA.BindingID, mA.MissionID, sA.SourceID)
	rel := MandateRelations{Principal: pA, Agent: aA, Issuer: iA, Mission: mA, Source: sA}

	t.Run("principal", func(t *testing.T) {
		p := base
		p.PrincipalBindingID = pB.BindingID
		rel := rel
		rel.Principal = pB
		_, err := NewMandateWithRelations(p, rel)
		if !errors.Is(err, ErrCrossTenant) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("agent", func(t *testing.T) {
		p := base
		p.AgentBindingID = aB.BindingID
		rel := rel
		rel.Agent = aB
		_, err := NewMandateWithRelations(p, rel)
		if !errors.Is(err, ErrCrossTenant) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("mission", func(t *testing.T) {
		p := base
		p.MissionID = mB.MissionID
		rel := rel
		rel.Mission = mB
		_, err := NewMandateWithRelations(p, rel)
		if !errors.Is(err, ErrCrossTenant) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("authority source", func(t *testing.T) {
		p := base
		p.AuthoritySourceID = sB.SourceID
		rel := rel
		rel.Source = sB
		_, err := NewMandateWithRelations(p, rel)
		if !errors.Is(err, ErrCrossTenant) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("parent mandate", func(t *testing.T) {
		p := base
		p.ParentMandateID = ptrID(parentB.MandateID)
		rel := rel
		rel.Parent = &parentB
		_, err := NewMandateWithRelations(p, rel)
		if !errors.Is(err, ErrCrossTenant) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestMandateRejectsWrongBindingKinds(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	mission := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)
	p := validMandateParams(org.OrganizationID, agent.BindingID, principal.BindingID, issuer.BindingID, mission.MissionID, source.SourceID)
	_, err := NewMandateWithRelations(p, MandateRelations{
		Principal: agent, Agent: principal, Issuer: issuer, Mission: mission, Source: source,
	})
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("err = %v, want ErrInvalidReference", err)
	}
}

func TestSameOrganization(t *testing.T) {
	a := uuid.Must(uuid.NewV7())
	b := uuid.Must(uuid.NewV7())
	if err := SameOrganization(a, a); err != nil {
		t.Fatalf("same org: %v", err)
	}
	if err := SameOrganization(a, b); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("err = %v, want ErrCrossTenant", err)
	}
	if err := SameOrganization(uuid.Nil, a); !errors.Is(err, ErrRequiredField) {
		t.Fatalf("err = %v, want ErrRequiredField", err)
	}
}
