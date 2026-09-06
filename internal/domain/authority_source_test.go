package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewAuthoritySource(t *testing.T) {
	org := testOrg(t, "Org")
	steward := testBinding(t, org.OrganizationID, KindPrincipal, "steward-1")
	src, err := NewAuthoritySourceWithRelations(AuthoritySourceParams{
		OrganizationID:   org.OrganizationID,
		Type:             SourcePolicy,
		StewardBindingID: steward.BindingID,
		ExternalRef:      "https://policy.example/grant/1",
		EvidencePointer:  "urn:evidence:1",
		Summary:          "Board policy 12",
	}, AuthoritySourceRelations{Steward: steward})
	if err != nil {
		t.Fatalf("NewAuthoritySourceWithRelations: %v", err)
	}
	if src.Type != SourcePolicy {
		t.Fatalf("type = %q", src.Type)
	}
	if src.StewardBindingID != steward.BindingID {
		t.Fatal("steward mismatch")
	}
}

func TestNewAuthoritySourceMissingFields(t *testing.T) {
	orgID := testOrg(t, "Org").OrganizationID
	steward := uuid.Must(uuid.NewV7())
	cases := []struct {
		name string
		p    AuthoritySourceParams
		want error
	}{
		{
			name: "missing organization",
			p: AuthoritySourceParams{
				Type: SourceRole, StewardBindingID: steward, ExternalRef: "ref", Summary: "s",
			},
			want: ErrRequiredField,
		},
		{
			name: "missing steward",
			p: AuthoritySourceParams{
				OrganizationID: orgID, Type: SourceRole, ExternalRef: "ref", Summary: "s",
			},
			want: ErrRequiredField,
		},
		{
			name: "missing external_ref",
			p: AuthoritySourceParams{
				OrganizationID: orgID, Type: SourceRole, StewardBindingID: steward, Summary: "s",
			},
			want: ErrRequiredField,
		},
		{
			name: "missing summary",
			p: AuthoritySourceParams{
				OrganizationID: orgID, Type: SourceRole, StewardBindingID: steward, ExternalRef: "ref",
			},
			want: ErrRequiredField,
		},
		{
			name: "invalid type",
			p: AuthoritySourceParams{
				OrganizationID: orgID, Type: "whisper", StewardBindingID: steward, ExternalRef: "ref", Summary: "s",
			},
			want: ErrInvalidInput,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthoritySource(tc.p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAuthoritySourceRejectsCrossTenantSteward(t *testing.T) {
	orgA := testOrg(t, "A")
	orgB := testOrg(t, "B")
	steward := testBinding(t, orgB.OrganizationID, KindPrincipal, "other-steward")
	_, err := NewAuthoritySourceWithRelations(AuthoritySourceParams{
		OrganizationID:   orgA.OrganizationID,
		Type:             SourceContract,
		StewardBindingID: steward.BindingID,
		ExternalRef:      "contract://1",
		Summary:          "cross tenant",
	}, AuthoritySourceRelations{Steward: steward})
	if !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("err = %v, want ErrCrossTenant", err)
	}
}
