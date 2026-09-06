package domain

import (
	"errors"
	"testing"
	"time"
)

func TestMissionTransitions(t *testing.T) {
	now := testNow()
	cases := []struct {
		name    string
		from    MissionState
		to      MissionState
		wantErr error
	}{
		{"draft to pending", MissionDraft, MissionPendingApproval, nil},
		{"pending to approved", MissionPendingApproval, MissionApproved, nil},
		{"approved to suspended", MissionApproved, MissionSuspended, nil},
		{"approved to ended", MissionApproved, MissionEnded, nil},
		{"draft to ended", MissionDraft, MissionEnded, nil},
		{"pending to ended", MissionPendingApproval, MissionEnded, nil},
		{"suspended to ended", MissionSuspended, MissionEnded, nil},
		{"draft cannot skip to approved", MissionDraft, MissionApproved, ErrInvalidTransition},
		{"approved cannot return to draft", MissionApproved, MissionDraft, ErrInvalidTransition},
		{"suspended cannot resume", MissionSuspended, MissionApproved, ErrInvalidTransition},
		{"ended is terminal", MissionEnded, MissionApproved, ErrInvalidTransition},
		{"expired is terminal", MissionExpired, MissionEnded, ErrInvalidTransition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := testMissionInState(t, tc.from)
			got, err := TransitionMission(m, tc.to, now, nil)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("TransitionMission: %v", err)
			}
			if got.State != tc.to {
				t.Fatalf("state = %s, want %s", got.State, tc.to)
			}
		})
	}
}

func TestMissionExpireOnlyAfterInclusiveExpiry(t *testing.T) {
	m := testMissionInState(t, MissionApproved)
	_, err := TransitionMission(m, MissionExpired, m.Expiry, nil)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expire at inclusive expiry: %v", err)
	}
	got, err := TransitionMission(m, MissionExpired, m.Expiry.Add(time.Second), nil)
	if err != nil {
		t.Fatalf("expire after expiry: %v", err)
	}
	if got.State != MissionExpired {
		t.Fatalf("state = %s", got.State)
	}
}

func TestApproveSubMissionRequiresApprovedParent(t *testing.T) {
	now := testNow()
	parent := testMissionInState(t, MissionApproved)
	child := testSubMission(t, parent, MissionPendingApproval)
	got, err := TransitionMission(child, MissionApproved, now, &parent)
	if err != nil {
		t.Fatalf("approve under approved parent: %v", err)
	}
	if got.State != MissionApproved {
		t.Fatal("expected approved")
	}

	draftParent := parent
	draftParent.State = MissionDraft
	_, err = TransitionMission(child, MissionApproved, now, &draftParent)
	if !errors.Is(err, ErrMissionNotApproved) {
		t.Fatalf("err = %v, want ErrMissionNotApproved", err)
	}

	_, err = TransitionMission(child, MissionApproved, now, nil)
	if !errors.Is(err, ErrBrokenChain) {
		t.Fatalf("missing parent: %v", err)
	}
}

func TestApproveRejectedOutsideWindow(t *testing.T) {
	m := testMissionInState(t, MissionPendingApproval)
	_, err := TransitionMission(m, MissionApproved, m.NotBefore.Add(-time.Second), nil)
	if !errors.Is(err, ErrMissionOutOfWindow) {
		t.Fatalf("before window: %v", err)
	}
	_, err = TransitionMission(m, MissionApproved, m.Expiry.Add(time.Second), nil)
	if !errors.Is(err, ErrMissionOutOfWindow) {
		t.Fatalf("after window: %v", err)
	}
}

func TestAssertAuthorizes(t *testing.T) {
	now := testNow()
	approved := testMissionInState(t, MissionApproved)
	if err := approved.AssertAuthorizes(now); err != nil {
		t.Fatalf("approved in-window: %v", err)
	}
	for _, state := range []MissionState{MissionDraft, MissionPendingApproval, MissionSuspended, MissionEnded, MissionExpired} {
		m := testMissionInState(t, state)
		if err := m.AssertAuthorizes(now); !errors.Is(err, ErrMissionNotApproved) {
			t.Fatalf("state %s: err = %v", state, err)
		}
	}
	if err := approved.AssertAuthorizes(approved.NotBefore.Add(-time.Second)); !errors.Is(err, ErrMissionOutOfWindow) {
		t.Fatalf("before not_before: %v", err)
	}
	if err := approved.AssertAuthorizes(approved.Expiry.Add(time.Second)); !errors.Is(err, ErrMissionOutOfWindow) {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestSubMissionCannotOutliveParent(t *testing.T) {
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	parent := testMission(t, org.OrganizationID, principal.BindingID, source.SourceID)
	nb, exp := testWindow()
	_, err := NewMissionWithRelations(MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "child",
		IntendedOutcome:    "child outcome",
		ParentMissionID:    ptrID(parent.MissionID),
		NotBefore:          nb,
		Expiry:             exp.Add(time.Hour),
		State:              MissionDraft,
		AuthoritySourceID:  source.SourceID,
	}, MissionRelations{Principal: principal, Source: source, Parent: &parent})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("outliving parent: %v", err)
	}
}

func testMissionInState(t *testing.T, state MissionState) Mission {
	t.Helper()
	org := testOrg(t, "Org")
	principal := testBinding(t, org.OrganizationID, KindPrincipal, "p")
	source := testSource(t, org.OrganizationID, principal.BindingID)
	nb, exp := testWindow()
	m, err := NewMission(MissionParams{
		OrganizationID:     org.OrganizationID,
		PrincipalBindingID: principal.BindingID,
		Purpose:            "purpose",
		IntendedOutcome:    "outcome",
		NotBefore:          nb,
		Expiry:             exp,
		State:              state,
		AuthoritySourceID:  source.SourceID,
	})
	if err != nil {
		t.Fatalf("NewMission: %v", err)
	}
	return m
}

func testSubMission(t *testing.T, parent Mission, state MissionState) Mission {
	t.Helper()
	m, err := NewMission(MissionParams{
		OrganizationID:     parent.OrganizationID,
		PrincipalBindingID: parent.PrincipalBindingID,
		Purpose:            "sub purpose",
		IntendedOutcome:    "sub outcome",
		ParentMissionID:    ptrID(parent.MissionID),
		NotBefore:          parent.NotBefore,
		Expiry:             parent.Expiry,
		State:              state,
		AuthoritySourceID:  parent.AuthoritySourceID,
	})
	if err != nil {
		t.Fatalf("NewMission sub: %v", err)
	}
	return m
}
