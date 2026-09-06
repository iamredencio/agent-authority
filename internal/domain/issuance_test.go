package domain

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type issuanceEnv struct {
	org       Organization
	principal IdentityBinding
	agent     IdentityBinding
	issuer    IdentityBinding
	source    AuthoritySource
	mission   Mission
}

func newIssuanceEnv(t *testing.T, missionState MissionState) issuanceEnv {
	t.Helper()
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	agent := testBinding(t, org.OrganizationID, KindAgent, "a")
	issuer := testBinding(t, org.OrganizationID, KindIssuer, "i")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	nb, exp := testWindow()
	mission, err := NewMission(MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "purpose",
		IntendedOutcome:    "outcome",
		NotBefore:          nb,
		Expiry:             exp,
		State:              missionState,
		AuthoritySourceID:  source.SourceID,
	})
	if err != nil {
		t.Fatalf("NewMission: %v", err)
	}
	return issuanceEnv{org, principal, agent, issuer, source, mission}
}

func (e issuanceEnv) relations() MandateRelations {
	return MandateRelations{
		Principal: e.principal,
		Agent:     e.agent,
		Issuer:    e.issuer,
		Mission:   e.mission,
		Source:    e.source,
	}
}

func (e issuanceEnv) parentParams() MandateParams {
	return richMandateParams(e.org.OrganizationID, e.principal.BindingID, e.agent.BindingID, e.issuer.BindingID, e.mission.MissionID, e.source.SourceID)
}

func TestIssueOriginatingMandateApprovedInWindow(t *testing.T) {
	env := newIssuanceEnv(t, MissionApproved)
	got, err := IssueOriginatingMandate(env.parentParams(), env.relations(), testNow())
	if err != nil {
		t.Fatalf("IssueOriginatingMandate: %v", err)
	}
	if got.ParentMandateID != nil {
		t.Fatal("originating mandate has a parent")
	}
	if got.State != MandateActive {
		t.Fatalf("state = %s", got.State)
	}
}

func TestIssueOriginatingMandateRejectsNonApprovedMissions(t *testing.T) {
	for _, state := range []MissionState{MissionDraft, MissionPendingApproval, MissionSuspended, MissionEnded, MissionExpired} {
		t.Run(string(state), func(t *testing.T) {
			env := newIssuanceEnv(t, state)
			_, err := IssueOriginatingMandate(env.parentParams(), env.relations(), testNow())
			if !errors.Is(err, ErrMissionNotApproved) {
				t.Fatalf("err = %v, want ErrMissionNotApproved", err)
			}
		})
	}
}

func TestIssueOriginatingMandateRejectsOutOfWindow(t *testing.T) {
	env := newIssuanceEnv(t, MissionApproved)
	_, err := IssueOriginatingMandate(env.parentParams(), env.relations(), env.mission.NotBefore.Add(-time.Second))
	if !errors.Is(err, ErrMissionOutOfWindow) {
		t.Fatalf("before not_before: %v", err)
	}
	_, err = IssueOriginatingMandate(env.parentParams(), env.relations(), env.mission.Expiry.Add(time.Second))
	if !errors.Is(err, ErrMissionOutOfWindow) {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestIssueOriginatingMandateRejectsParent(t *testing.T) {
	env := newIssuanceEnv(t, MissionApproved)
	p := env.parentParams()
	id := uuid.Must(uuid.NewV7())
	p.ParentMandateID = &id
	_, err := IssueOriginatingMandate(p, env.relations(), testNow())
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestIssueChildEqualOnAxis(t *testing.T) {
	parent, env := issueParent(t)
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.DelegationDepth = parent.DelegationDepth - 1
	rel := env.relations()
	rel.Parent = &parent
	got, err := IssueChildMandate(childP, rel, env.mission, nil, testNow())
	if err != nil {
		t.Fatalf("equal-on-axis child: %v", err)
	}
	if got.DelegationDepth != 1 {
		t.Fatalf("depth = %d", got.DelegationDepth)
	}
}

func TestIssueChildNarrowerAuthority(t *testing.T) {
	parent, env := issueParent(t)
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.DelegationDepth = 0
	childP.CommunicationAuthority = json.RawMessage(`[{"destination":"https://finance.example"}]`)
	childP.ExecutionAuthority = json.RawMessage(`[{"action":"draft_report"}]`)
	childP.Budget = json.RawMessage(`{"unit":"EUR","amount":"25.00","remaining":"25.00","categories":["travel"]}`)
	childP.Constraints = json.RawMessage(`{"geo":"NL","rate":1}`)
	childP.EvidenceRequirements = json.RawMessage(`["call.api","call.payment"]`)
	childP.ApprovalRequirements = json.RawMessage(`{"human":true,"count":2}`)
	rel := env.relations()
	rel.Parent = &parent
	if _, err := IssueChildMandate(childP, rel, env.mission, nil, testNow()); err != nil {
		t.Fatalf("narrower child: %v", err)
	}
}

func TestIssueChildRejectsEachWidenedAxis(t *testing.T) {
	parent, env := issueParent(t)
	base := env.parentParams()
	base.ParentMandateID = ptrID(parent.MandateID)
	base.DelegationDepth = 1
	rel := env.relations()
	rel.Parent = &parent

	cases := []struct {
		name string
		mut  func(*MandateParams)
	}{
		{"scope", func(p *MandateParams) { p.Scope = json.RawMessage(`{"class":"reporting","extra":"audit"}`) }},
		{"communication", func(p *MandateParams) {
			p.CommunicationAuthority = json.RawMessage(`[{"destination":"https://finance.example"},{"destination":"https://other.example"}]`)
		}},
		{"execution", func(p *MandateParams) {
			p.ExecutionAuthority = json.RawMessage(`[{"action":"draft_report"},{"action":"pay"}]`)
		}},
		{"constraints", func(p *MandateParams) { p.Constraints = json.RawMessage(`{}`) }},
		{"budget amount", func(p *MandateParams) {
			p.Budget = json.RawMessage(`{"unit":"EUR","amount":"80.00","remaining":"80.00","categories":["travel"]}`)
		}},
		{"budget category", func(p *MandateParams) {
			p.Budget = json.RawMessage(`{"unit":"EUR","amount":"10.00","remaining":"10.00","categories":["travel","meals"]}`)
		}},
		{"budget unit", func(p *MandateParams) {
			p.Budget = json.RawMessage(`{"unit":"USD","amount":"10.00","remaining":"10.00","categories":["travel"]}`)
		}},
		{"expiry", func(p *MandateParams) { p.Expiry = parent.Expiry.Add(time.Hour) }},
		{"not_before", func(p *MandateParams) { p.NotBefore = parent.NotBefore.Add(-time.Hour) }},
		{"delegation_depth equal", func(p *MandateParams) { p.DelegationDepth = parent.DelegationDepth }},
		{"approval", func(p *MandateParams) { p.ApprovalRequirements = json.RawMessage(`{}`) }},
		{"evidence", func(p *MandateParams) { p.EvidenceRequirements = json.RawMessage(`[]`) }},
		{"authority source", func(p *MandateParams) { p.AuthoritySourceID = uuid.Must(uuid.NewV7()) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mut(&p)
			if tc.name == "authority source" {
				other := testSource(t, env.org.OrganizationID, env.principal.BindingID)
				p.AuthoritySourceID = other.SourceID
				rel := rel
				rel.Source = other
				_, err := IssueChildMandate(p, rel, env.mission, nil, testNow())
				if !errors.Is(err, ErrAmplification) {
					t.Fatalf("err = %v, want ErrAmplification", err)
				}
				return
			}
			_, err := IssueChildMandate(p, rel, env.mission, nil, testNow())
			if err == nil {
				t.Fatal("expected widening to fail")
			}
			if !errors.Is(err, ErrAmplification) && !errors.Is(err, ErrInvalidInput) && !errors.Is(err, ErrInvalidReference) {
				t.Fatalf("err = %v, want amplification or window/reference failure", err)
			}
		})
	}
}

func TestParentDepthZeroCannotDelegate(t *testing.T) {
	env := newIssuanceEnv(t, MissionApproved)
	p := env.parentParams()
	p.DelegationDepth = 0
	parent, err := IssueOriginatingMandate(p, env.relations(), testNow())
	if err != nil {
		t.Fatalf("originating depth 0: %v", err)
	}
	childP := p
	childP.MandateID = uuid.Nil
	childP.ParentMandateID = ptrID(parent.MandateID)
	rel := env.relations()
	rel.Parent = &parent
	_, err = IssueChildMandate(childP, rel, env.mission, nil, testNow())
	if !errors.Is(err, ErrCannotDelegate) {
		t.Fatalf("err = %v, want ErrCannotDelegate", err)
	}
}

func TestChildTimeWindowInsideParent(t *testing.T) {
	parent, env := issueParent(t)
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.DelegationDepth = 1
	childP.NotBefore = parent.NotBefore.Add(time.Hour)
	childP.Expiry = parent.Expiry.Add(-time.Hour)
	rel := env.relations()
	rel.Parent = &parent
	if _, err := IssueChildMandate(childP, rel, env.mission, nil, testNow()); err != nil {
		t.Fatalf("inside parent window: %v", err)
	}
}

func TestChildTimeWindowOutsideParentFails(t *testing.T) {
	parent, env := issueParent(t)
	rel := env.relations()
	rel.Parent = &parent
	over := env.parentParams()
	over.ParentMandateID = ptrID(parent.MandateID)
	over.DelegationDepth = 1
	over.Expiry = parent.Expiry.Add(time.Minute)
	_, err := IssueChildMandate(over, rel, env.mission, nil, testNow())
	if err == nil {
		t.Fatal("expected expiry overflow to fail")
	}
	early := env.parentParams()
	early.ParentMandateID = ptrID(parent.MandateID)
	early.DelegationDepth = 1
	early.NotBefore = parent.NotBefore.Add(-time.Minute)
	_, err = IssueChildMandate(early, rel, env.mission, nil, testNow())
	if err == nil {
		t.Fatal("expected not_before overflow to fail")
	}
}

func TestCrossTenantParentChildFails(t *testing.T) {
	parent, _ := issueParent(t)
	other := newIssuanceEnv(t, MissionApproved)
	childP := other.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.DelegationDepth = 1
	rel := other.relations()
	rel.Parent = &parent
	_, err := IssueChildMandate(childP, rel, other.mission, nil, testNow())
	if !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("err = %v, want ErrCrossTenant", err)
	}
}

func TestIssueChildOnApprovedSubMission(t *testing.T) {
	parent, env := issueParent(t)
	sub := testSubMission(t, env.mission, MissionApproved)
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.MissionID = sub.MissionID
	childP.DelegationDepth = 1
	rel := env.relations()
	rel.Parent = &parent
	rel.Mission = sub
	if _, err := IssueChildMandate(childP, rel, env.mission, &env.mission, testNow()); err != nil {
		t.Fatalf("sub-mission child: %v", err)
	}
}

func TestIssueChildRejectsUnrelatedMission(t *testing.T) {
	parent, env := issueParent(t)
	other := testMissionInState(t, MissionApproved)
	other.OrganizationID = env.org.OrganizationID
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.MissionID = other.MissionID
	childP.DelegationDepth = 1
	rel := env.relations()
	rel.Parent = &parent
	rel.Mission = other
	_, err := IssueChildMandate(childP, rel, env.mission, nil, testNow())
	if err == nil {
		t.Fatal("unrelated mission should fail")
	}
}

func TestMalformedAuthorityFailsClosed(t *testing.T) {
	parent, env := issueParent(t)
	rel := env.relations()
	rel.Parent = &parent
	cases := []struct {
		name string
		mut  func(*MandateParams)
	}{
		{"communication object", func(p *MandateParams) {
			p.CommunicationAuthority = json.RawMessage(`{"destination":"https://finance.example"}`)
		}},
		{"wildcard destination", func(p *MandateParams) { p.CommunicationAuthority = json.RawMessage(`[{"destination":"*"}]`) }},
		{"missing destination", func(p *MandateParams) { p.CommunicationAuthority = json.RawMessage(`[{"class":"http"}]`) }},
		{"budget extra field", func(p *MandateParams) {
			p.Budget = json.RawMessage(`{"unit":"EUR","amount":"1","remaining":"1","categories":[],"bonus":true}`)
		}},
		{"unknown constraint change", func(p *MandateParams) { p.Constraints = json.RawMessage(`{"geo":"NL","custom":"changed"}`) }},
		{"approval as string", func(p *MandateParams) { p.ApprovalRequirements = json.RawMessage(`"human"`) }},
		{"evidence as number", func(p *MandateParams) { p.EvidenceRequirements = json.RawMessage(`1`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := env.parentParams()
			p.ParentMandateID = ptrID(parent.MandateID)
			p.DelegationDepth = 1
			if tc.name == "unknown constraint change" {
				p.Constraints = json.RawMessage(`{"geo":"NL","custom":"changed"}`)
			} else {
				tc.mut(&p)
			}
			_, err := IssueChildMandate(p, rel, env.mission, nil, testNow())
			if err == nil {
				t.Fatal("expected malformed authority to fail closed")
			}
			if !errors.Is(err, ErrMalformedAuthority) && !errors.Is(err, ErrAmplification) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestNewMandateStillStoresParentWithoutIssuanceEngine(t *testing.T) {
	env := newIssuanceEnv(t, MissionApproved)
	parent := testMandate(t, env.org.OrganizationID, env.principal.BindingID, env.agent.BindingID, env.issuer.BindingID, env.mission.MissionID, env.source.SourceID)
	p := validMandateParams(env.org.OrganizationID, env.principal.BindingID, env.agent.BindingID, env.issuer.BindingID, env.mission.MissionID, env.source.SourceID)
	p.ParentMandateID = ptrID(parent.MandateID)
	p.DelegationDepth = parent.DelegationDepth
	_, err := NewMandateWithRelations(p, MandateRelations{
		Principal: env.principal, Agent: env.agent, Issuer: env.issuer, Mission: env.mission, Source: env.source, Parent: &parent,
	})
	if err != nil {
		t.Fatalf("Phase 1 constructor must still store parent without attenuation: %v", err)
	}
}

func issueParent(t *testing.T) (Mandate, issuanceEnv) {
	t.Helper()
	env := newIssuanceEnv(t, MissionApproved)
	parent, err := IssueOriginatingMandate(env.parentParams(), env.relations(), testNow())
	if err != nil {
		t.Fatalf("parent issuance: %v", err)
	}
	return parent, env
}

func richMandateParams(orgID, principal, agent, issuer, mission, source uuid.UUID) MandateParams {
	nb, exp := testWindow()
	return MandateParams{
		Version:                MandateSchemaVersion,
		OrganizationID:         orgID,
		PrincipalBindingID:     principal,
		AgentBindingID:         agent,
		MissionID:              mission,
		AuthoritySourceID:      source,
		Scope:                  json.RawMessage(`{"class":"reporting"}`),
		CommunicationAuthority: json.RawMessage(`[{"destination":"https://finance.example"},{"destination":"https://hr.example"}]`),
		ExecutionAuthority:     json.RawMessage(`[{"action":"draft_report"},{"action":"read_ledger"}]`),
		Constraints:            json.RawMessage(`{"geo":"NL"}`),
		Budget:                 json.RawMessage(`{"unit":"EUR","amount":"50.00","remaining":"50.00","categories":["travel"]}`),
		NotBefore:              nb,
		Expiry:                 exp,
		IssuedAt:               testNow(),
		DelegationDepth:        2,
		ApprovalRequirements:   json.RawMessage(`{"human":true}`),
		EvidenceRequirements:   json.RawMessage(`["call.api"]`),
		IssuerBindingID:        issuer,
		State:                  MandateActive,
	}
}
