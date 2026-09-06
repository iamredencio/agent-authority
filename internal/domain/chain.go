package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const maxDelegationChainHops = 128

// MandateLookup loads one organization-scoped mandate. Implementations must
// fail closed on a missing or cross-tenant identifier.
type MandateLookup func(organizationID, mandateID uuid.UUID) (Mandate, error)

// MissionLookup loads one organization-scoped mission. Implementations must
// fail closed on a missing or cross-tenant identifier.
type MissionLookup func(organizationID, missionID uuid.UUID) (Mission, error)

// ReconstructDelegationChain is a structural walk of parent_mandate_id from
// actor to origin. It proves that links exist, are same-tenant, acyclic,
// structurally valid, and active. It does not prove attenuation and MUST NOT
// be treated as an authority decision. Use VerifyDelegationChain for that.
func ReconstructDelegationChain(organizationID, mandateID uuid.UUID, lookup MandateLookup) ([]Mandate, error) {
	if lookup == nil {
		return nil, fmt.Errorf("%w: mandate lookup required", ErrBrokenChain)
	}
	if err := requireID("organization_id", organizationID); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrokenChain, err)
	}
	if err := requireID("mandate_id", mandateID); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrokenChain, err)
	}

	seen := make(map[uuid.UUID]struct{}, 8)
	var reverse []Mandate
	currentID := mandateID
	for hop := 0; hop < maxDelegationChainHops; hop++ {
		if _, ok := seen[currentID]; ok {
			return nil, fmt.Errorf("%w: cycle at %s", ErrBrokenChain, currentID)
		}
		seen[currentID] = struct{}{}
		m, err := lookup(organizationID, currentID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBrokenChain, err)
		}
		if err := m.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBrokenChain, err)
		}
		if err := SameOrganization(organizationID, m.OrganizationID); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBrokenChain, err)
		}
		if m.State != MandateActive {
			return nil, fmt.Errorf("%w: ancestor %s state=%s", ErrBrokenChain, m.MandateID, m.State)
		}
		reverse = append(reverse, m)
		if m.ParentMandateID == nil {
			break
		}
		if *m.ParentMandateID == uuid.Nil {
			return nil, fmt.Errorf("%w: invalid parent_mandate_id", ErrBrokenChain)
		}
		currentID = *m.ParentMandateID
		if hop == maxDelegationChainHops-1 {
			return nil, fmt.Errorf("%w: chain exceeded %d hops", ErrBrokenChain, maxDelegationChainHops)
		}
	}
	if len(reverse) == 0 {
		return nil, fmt.Errorf("%w: empty chain", ErrBrokenChain)
	}
	out := make([]Mandate, len(reverse))
	for i, m := range reverse {
		out[len(reverse)-1-i] = m
	}
	if out[0].ParentMandateID != nil {
		return nil, fmt.Errorf("%w: origin still has a parent", ErrBrokenChain)
	}
	return out, nil
}

// VerifyDelegationChain is the authority-bearing reconstruction. It first
// walks the stored parent links, then proves every child→parent hop against
// the existing Phase 2 attenuation rules and that each mandate is usable now.
func VerifyDelegationChain(organizationID, mandateID uuid.UUID, mandates MandateLookup, missions MissionLookup, now time.Time) ([]Mandate, error) {
	if missions == nil {
		return nil, fmt.Errorf("%w: mission lookup required", ErrBrokenChain)
	}
	now = normalizeTime(now)
	if err := requireTime("now", now); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrokenChain, err)
	}
	chain, err := ReconstructDelegationChain(organizationID, mandateID, mandates)
	if err != nil {
		return nil, err
	}
	for i, m := range chain {
		mission, err := loadChainMission(organizationID, m.MissionID, missions)
		if err != nil {
			return nil, err
		}
		if err := AssertMandateUsable(m, mission, now); err != nil {
			return nil, fmt.Errorf("%w: mandate %s: %w", ErrBrokenChain, m.MandateID, err)
		}
		if i == 0 {
			continue
		}
		parent := chain[i-1]
		parentMission, err := loadChainMission(organizationID, parent.MissionID, missions)
		if err != nil {
			return nil, err
		}
		var childMissionParent *Mission
		if mission.ParentMissionID != nil {
			got, err := loadChainMission(organizationID, *mission.ParentMissionID, missions)
			if err != nil {
				return nil, err
			}
			childMissionParent = &got
		}
		if err := AssertAttenuation(m, parent, mission, parentMission, childMissionParent, now); err != nil {
			return nil, fmt.Errorf("%w: child %s: %w", ErrBrokenChain, m.MandateID, err)
		}
	}
	return chain, nil
}

func loadChainMission(organizationID, missionID uuid.UUID, lookup MissionLookup) (Mission, error) {
	m, err := lookup(organizationID, missionID)
	if err != nil {
		return Mission{}, fmt.Errorf("%w: mission %s: %w", ErrBrokenChain, missionID, err)
	}
	if err := m.Validate(); err != nil {
		return Mission{}, fmt.Errorf("%w: mission %s: %w", ErrBrokenChain, missionID, err)
	}
	if err := SameOrganization(organizationID, m.OrganizationID); err != nil {
		return Mission{}, fmt.Errorf("%w: mission %s: %w", ErrBrokenChain, missionID, err)
	}
	return m, nil
}
