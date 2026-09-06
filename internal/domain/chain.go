package domain

import (
	"fmt"

	"github.com/google/uuid"
)

const maxDelegationChainHops = 128

// MandateLookup loads one organization-scoped mandate. Implementations must
// fail closed on a missing or cross-tenant identifier.
type MandateLookup func(organizationID, mandateID uuid.UUID) (Mandate, error)

// ReconstructDelegationChain walks parent_mandate_id from actor to origin.
// The result is origin-first. Reconstruction fails closed if any link is
// missing, cross-tenant, cyclic, or not an active ancestor.
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
