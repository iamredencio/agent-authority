package domain

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestReconstructDelegationChainSuccess(t *testing.T) {
	origin, env := issueParent(t)
	midP := env.parentParams()
	midP.ParentMandateID = ptrID(origin.MandateID)
	midP.DelegationDepth = 1
	rel := env.relations()
	rel.Parent = &origin
	mid, err := IssueChildMandate(midP, rel, env.mission, nil, testNow())
	if err != nil {
		t.Fatalf("mid: %v", err)
	}
	leafP := env.parentParams()
	leafP.ParentMandateID = ptrID(mid.MandateID)
	leafP.DelegationDepth = 0
	rel.Parent = &mid
	leaf, err := IssueChildMandate(leafP, rel, env.mission, nil, testNow())
	if err != nil {
		t.Fatalf("leaf: %v", err)
	}

	lookup := mapLookup(origin, mid, leaf)
	chain, err := ReconstructDelegationChain(env.org.OrganizationID, leaf.MandateID, lookup)
	if err != nil {
		t.Fatalf("ReconstructDelegationChain: %v", err)
	}
	if len(chain) != 3 {
		t.Fatalf("len = %d", len(chain))
	}
	if chain[0].MandateID != origin.MandateID || chain[1].MandateID != mid.MandateID || chain[2].MandateID != leaf.MandateID {
		t.Fatalf("order = %v → %v → %v", chain[0].MandateID, chain[1].MandateID, chain[2].MandateID)
	}
	verified, err := VerifyDelegationChain(env.org.OrganizationID, leaf.MandateID, lookup, mapMissionLookup(env.mission), testNow())
	if err != nil {
		t.Fatalf("VerifyDelegationChain: %v", err)
	}
	if len(verified) != 3 {
		t.Fatalf("verified len = %d", len(verified))
	}
}

func TestReconstructDelegationChainMissingLinkFailsClosed(t *testing.T) {
	origin, env := issueParent(t)
	missingParent := uuid.Must(uuid.NewV7())
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(missingParent)
	childP.DelegationDepth = 1
	child, err := NewMandate(childP)
	if err != nil {
		t.Fatalf("NewMandate: %v", err)
	}
	lookup := func(org, id uuid.UUID) (Mandate, error) {
		if id == child.MandateID {
			return child, nil
		}
		if id == origin.MandateID {
			return origin, nil
		}
		return Mandate{}, ErrNotFound
	}
	_, err = ReconstructDelegationChain(env.org.OrganizationID, child.MandateID, lookup)
	if !errors.Is(err, ErrBrokenChain) {
		t.Fatalf("err = %v, want ErrBrokenChain", err)
	}
}

func TestReconstructDelegationChainCrossTenantFailsClosed(t *testing.T) {
	origin, env := issueParent(t)
	other := uuid.Must(uuid.NewV7())
	lookup := func(org, id uuid.UUID) (Mandate, error) {
		if org != env.org.OrganizationID {
			return Mandate{}, ErrNotFound
		}
		if id == origin.MandateID {
			return origin, nil
		}
		return Mandate{}, ErrNotFound
	}
	_, err := ReconstructDelegationChain(other, origin.MandateID, lookup)
	if !errors.Is(err, ErrBrokenChain) {
		t.Fatalf("err = %v, want ErrBrokenChain", err)
	}
}

func TestReconstructDelegationChainRevokedAncestorFailsClosed(t *testing.T) {
	origin, env := issueParent(t)
	origin.State = MandateRevoked
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(origin.MandateID)
	childP.DelegationDepth = 1
	child, err := NewMandate(childP)
	if err != nil {
		t.Fatalf("NewMandate: %v", err)
	}
	_, err = ReconstructDelegationChain(env.org.OrganizationID, child.MandateID, mapLookup(origin, child))
	if !errors.Is(err, ErrBrokenChain) {
		t.Fatalf("err = %v, want ErrBrokenChain", err)
	}
}

func TestReconstructDelegationChainCycleFailsClosed(t *testing.T) {
	env := newIssuanceEnv(t, MissionApproved)
	aID := uuid.Must(uuid.NewV7())
	bID := uuid.Must(uuid.NewV7())
	aP := env.parentParams()
	aP.MandateID = aID
	aP.ParentMandateID = ptrID(bID)
	bP := env.parentParams()
	bP.MandateID = bID
	bP.ParentMandateID = ptrID(aID)
	a, err := NewMandate(aP)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewMandate(bP)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ReconstructDelegationChain(env.org.OrganizationID, a.MandateID, mapLookup(a, b))
	if !errors.Is(err, ErrBrokenChain) {
		t.Fatalf("err = %v, want ErrBrokenChain", err)
	}
}

func TestVerifyDelegationChainRejectsWidenedStoredChild(t *testing.T) {
	parent, env := issueParent(t)
	childP := env.parentParams()
	childP.ParentMandateID = ptrID(parent.MandateID)
	childP.DelegationDepth = 1
	childP.ExecutionAuthority = json.RawMessage(`[{"action":"draft_report"},{"action":"wire_funds"}]`)
	child, err := NewMandate(childP)
	if err != nil {
		t.Fatalf("storage constructor must still persist a widened child: %v", err)
	}

	lookup := mapLookup(parent, child)
	chain, err := ReconstructDelegationChain(env.org.OrganizationID, child.MandateID, lookup)
	if err != nil {
		t.Fatalf("structural reconstruct of a widened child: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("len = %d", len(chain))
	}

	_, err = VerifyDelegationChain(env.org.OrganizationID, child.MandateID, lookup, mapMissionLookup(env.mission), testNow())
	if !errors.Is(err, ErrBrokenChain) || !errors.Is(err, ErrAmplification) {
		t.Fatalf("verified reconstruct err = %v, want ErrBrokenChain and ErrAmplification", err)
	}
}

func mapLookup(mandates ...Mandate) MandateLookup {
	byID := make(map[uuid.UUID]Mandate, len(mandates))
	for _, m := range mandates {
		byID[m.MandateID] = m
	}
	return func(org, id uuid.UUID) (Mandate, error) {
		m, ok := byID[id]
		if !ok || m.OrganizationID != org {
			return Mandate{}, ErrNotFound
		}
		return m, nil
	}
}

func mapMissionLookup(missions ...Mission) MissionLookup {
	byID := make(map[uuid.UUID]Mission, len(missions))
	for _, m := range missions {
		byID[m.MissionID] = m
	}
	return func(org, id uuid.UUID) (Mission, error) {
		m, ok := byID[id]
		if !ok || m.OrganizationID != org {
			return Mission{}, ErrNotFound
		}
		return m, nil
	}
}
