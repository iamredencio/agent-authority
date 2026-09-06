package domain

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// BindingKind is the role of an external identity reference inside one organization.
type BindingKind string

const (
	KindPrincipal BindingKind = "principal"
	KindAgent     BindingKind = "agent"
	KindIssuer    BindingKind = "issuer"
)

// IdentityProvider is an external identity issuer. Agent Authority is not an IdP.
type IdentityProvider string

const (
	ProviderEntra       IdentityProvider = "entra"
	ProviderOkta        IdentityProvider = "okta"
	ProviderSPIFFE      IdentityProvider = "spiffe"
	ProviderCrowdStrike IdentityProvider = "crowdstrike"
	ProviderWorkload    IdentityProvider = "workload"
	ProviderOIDC        IdentityProvider = "oidc"
	ProviderCustom      IdentityProvider = "custom"
)

// BindingStatus is the local record status of an external identity reference.
type BindingStatus string

const (
	BindingActive   BindingStatus = "active"
	BindingDisabled BindingStatus = "disabled"
)

// IdentityBinding stores a pointer to an identity mastered elsewhere.
// It is not a local user, credential, or identity provider record.
type IdentityBinding struct {
	BindingID       uuid.UUID
	OrganizationID  uuid.UUID
	Kind            BindingKind
	Provider        IdentityProvider
	Subject         string
	DisplayName     string
	AttestationMeta json.RawMessage
	Status          BindingStatus
}

type IdentityBindingParams struct {
	BindingID       uuid.UUID
	OrganizationID  uuid.UUID
	Kind            BindingKind
	Provider        IdentityProvider
	Subject         string
	DisplayName     string
	AttestationMeta json.RawMessage
	Status          BindingStatus
}

func NewIdentityBinding(p IdentityBindingParams) (IdentityBinding, error) {
	id := p.BindingID
	if id == uuid.Nil {
		generated, err := newID()
		if err != nil {
			return IdentityBinding{}, err
		}
		id = generated
	}
	status := p.Status
	if status == "" {
		status = BindingActive
	}
	b := IdentityBinding{
		BindingID:       id,
		OrganizationID:  p.OrganizationID,
		Kind:            p.Kind,
		Provider:        p.Provider,
		Subject:         p.Subject,
		DisplayName:     p.DisplayName,
		AttestationMeta: cloneJSON(p.AttestationMeta),
		Status:          status,
	}
	if err := b.Validate(); err != nil {
		return IdentityBinding{}, err
	}
	return b, nil
}

func (b IdentityBinding) Validate() error {
	if err := requireID("binding_id", b.BindingID); err != nil {
		return err
	}
	if err := requireID("organization_id", b.OrganizationID); err != nil {
		return err
	}
	if err := requireNonEmpty("subject", b.Subject); err != nil {
		return err
	}
	if !validBindingKind(b.Kind) {
		return fmt.Errorf("%w: kind", ErrInvalidInput)
	}
	if !validIdentityProvider(b.Provider) {
		return fmt.Errorf("%w: provider", ErrInvalidInput)
	}
	if !validBindingStatus(b.Status) {
		return fmt.Errorf("%w: status", ErrInvalidInput)
	}
	if len(b.AttestationMeta) > 0 {
		if err := requireJSON("attestation_meta", b.AttestationMeta); err != nil {
			return err
		}
	}
	return nil
}

func validBindingKind(k BindingKind) bool {
	switch k {
	case KindPrincipal, KindAgent, KindIssuer:
		return true
	default:
		return false
	}
}

func validIdentityProvider(p IdentityProvider) bool {
	switch p {
	case ProviderEntra, ProviderOkta, ProviderSPIFFE, ProviderCrowdStrike, ProviderWorkload, ProviderOIDC, ProviderCustom:
		return true
	default:
		return false
	}
}

func validBindingStatus(s BindingStatus) bool {
	switch s {
	case BindingActive, BindingDisabled:
		return true
	default:
		return false
	}
}

func cloneJSON(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}
