package domain

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewIdentityBindingExternalReference(t *testing.T) {
	org := testOrg(t, "Org")
	b, err := NewIdentityBinding(IdentityBindingParams{
		OrganizationID:  org.OrganizationID,
		Kind:            KindPrincipal,
		Provider:        ProviderEntra,
		Subject:         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		DisplayName:     "Ada",
		AttestationMeta: json.RawMessage(`{"issuer":"https://login.microsoftonline.com"}`),
		Status:          BindingActive,
	})
	if err != nil {
		t.Fatalf("NewIdentityBinding: %v", err)
	}
	if b.Provider != ProviderEntra {
		t.Fatalf("provider = %q", b.Provider)
	}
	if b.Subject == "" {
		t.Fatal("subject must be the external identifier")
	}
	if b.Kind != KindPrincipal {
		t.Fatalf("kind = %q", b.Kind)
	}

	typ := reflect.TypeOf(IdentityBinding{})
	forbidden := []string{"password", "secret", "credential", "hash", "username"}
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		for _, word := range forbidden {
			if strings.Contains(name, word) {
				t.Fatalf("IdentityBinding must remain an external reference; found field %s", typ.Field(i).Name)
			}
		}
	}
}

func TestNewIdentityBindingMissingFields(t *testing.T) {
	orgID := testOrg(t, "Org").OrganizationID
	cases := []struct {
		name string
		p    IdentityBindingParams
		want error
	}{
		{
			name: "missing organization",
			p: IdentityBindingParams{
				Kind: KindPrincipal, Provider: ProviderEntra, Subject: "sub-1",
			},
			want: ErrRequiredField,
		},
		{
			name: "missing subject",
			p: IdentityBindingParams{
				OrganizationID: orgID, Kind: KindPrincipal, Provider: ProviderOkta, Subject: " ",
			},
			want: ErrRequiredField,
		},
		{
			name: "invalid kind",
			p: IdentityBindingParams{
				OrganizationID: orgID, Kind: "user", Provider: ProviderOIDC, Subject: "sub-2",
			},
			want: ErrInvalidInput,
		},
		{
			name: "invalid provider",
			p: IdentityBindingParams{
				OrganizationID: orgID, Kind: KindAgent, Provider: "local", Subject: "sub-3",
			},
			want: ErrInvalidInput,
		},
		{
			name: "invalid status",
			p: IdentityBindingParams{
				OrganizationID: orgID, Kind: KindIssuer, Provider: ProviderCustom, Subject: "sub-4", Status: "unknown",
			},
			want: ErrInvalidInput,
		},
		{
			name: "invalid attestation json",
			p: IdentityBindingParams{
				OrganizationID: orgID, Kind: KindAgent, Provider: ProviderSPIFFE, Subject: "spiffe://example/agent",
				AttestationMeta: json.RawMessage(`not-json`),
			},
			want: ErrInvalidInput,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewIdentityBinding(tc.p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestIdentityBindingDoesNotMintLocalIdentity(t *testing.T) {
	org := testOrg(t, "Org")
	b := testBinding(t, org.OrganizationID, KindAgent, "workload://cluster/ns/pod")
	if b.Provider == "" || b.Subject == "" {
		t.Fatal("binding must record provider and subject")
	}
	if b.OrganizationID != org.OrganizationID {
		t.Fatal("binding must be organization-scoped")
	}
}

func TestIdentityBindingRejectsNilOrganizationID(t *testing.T) {
	err := IdentityBinding{
		BindingID: uuid.Must(uuid.NewV7()),
		Kind:      KindPrincipal,
		Provider:  ProviderEntra,
		Subject:   "sub",
		Status:    BindingActive,
	}.Validate()
	if !errors.Is(err, ErrRequiredField) {
		t.Fatalf("err = %v, want ErrRequiredField", err)
	}
}
