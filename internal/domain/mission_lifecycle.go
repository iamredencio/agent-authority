package domain

import (
	"fmt"
	"time"
)

// TransitionMission applies a documented mission state change.
//
// Allowed edges:
//
//	draft             → pending_approval | ended
//	pending_approval  → approved | ended
//	approved          → suspended | ended | expired
//	suspended         → ended | expired
//
// ended and expired are terminal. Resume from suspended is not specified
// and is rejected. draft → approved is rejected; approval must pass through
// pending_approval. expired is allowed only after the inclusive expiry instant.
func TransitionMission(m Mission, to MissionState, now time.Time, parent *Mission) (Mission, error) {
	if err := m.Validate(); err != nil {
		return Mission{}, err
	}
	if !validMissionState(to) {
		return Mission{}, fmt.Errorf("%w: state", ErrInvalidInput)
	}
	if m.State == to {
		return Mission{}, fmt.Errorf("%w: already %s", ErrInvalidTransition, to)
	}
	if !allowedMissionTransition(m.State, to) {
		return Mission{}, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, m.State, to)
	}
	now = normalizeTime(now)
	if err := requireTime("now", now); err != nil {
		return Mission{}, err
	}

	if to == MissionExpired && !now.After(m.Expiry) {
		return Mission{}, fmt.Errorf("%w: expiry is still inclusive at now", ErrInvalidTransition)
	}

	if to == MissionApproved {
		if !m.InWindow(now) {
			return Mission{}, fmt.Errorf("%w", ErrMissionOutOfWindow)
		}
		if parent != nil {
			if err := parent.Validate(); err != nil {
				return Mission{}, err
			}
			if m.ParentMissionID == nil || *m.ParentMissionID != parent.MissionID {
				return Mission{}, fmt.Errorf("%w: parent_mission_id", ErrInvalidReference)
			}
			if err := SameOrganization(m.OrganizationID, parent.OrganizationID); err != nil {
				return Mission{}, err
			}
			if err := parent.AssertAuthorizes(now); err != nil {
				return Mission{}, fmt.Errorf("%w: parent mission", err)
			}
			if err := m.validateWindowInside(*parent, "sub-mission"); err != nil {
				return Mission{}, err
			}
		} else if m.ParentMissionID != nil {
			return Mission{}, fmt.Errorf("%w: parent mission required to approve a sub-mission", ErrBrokenChain)
		}
	}

	m.State = to
	if err := m.Validate(); err != nil {
		return Mission{}, err
	}
	return m, nil
}

func allowedMissionTransition(from, to MissionState) bool {
	switch from {
	case MissionDraft:
		return to == MissionPendingApproval || to == MissionEnded
	case MissionPendingApproval:
		return to == MissionApproved || to == MissionEnded
	case MissionApproved:
		return to == MissionSuspended || to == MissionEnded || to == MissionExpired
	case MissionSuspended:
		return to == MissionEnded || to == MissionExpired
	default:
		return false
	}
}
