package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// AuthoritySourceType is the recorded origin class of a grant.
type AuthoritySourceType string

const (
	SourceHumanApproval      AuthoritySourceType = "human_approval"
	SourceRole               AuthoritySourceType = "role"
	SourcePolicy             AuthoritySourceType = "policy"
	SourceContract           AuthoritySourceType = "contract"
	SourceProcurementMandate AuthoritySourceType = "procurement_mandate"
	SourceBoardMandate       AuthoritySourceType = "board_mandate"
	SourceCredential         AuthoritySourceType = "credential"
	SourceMachineAgreement   AuthoritySourceType = "machine_agreement"
	SourceOther              AuthoritySourceType = "other"
)

// AuthoritySource is the traceable origin of authority. It is not itself a mandate.
type AuthoritySource struct {
	SourceID         uuid.UUID
	OrganizationID   uuid.UUID
	Type             AuthoritySourceType
	StewardBindingID uuid.UUID
	ExternalRef      string
	EvidencePointer  string
	Summary          string
}

type AuthoritySourceParams struct {
	SourceID         uuid.UUID
	OrganizationID   uuid.UUID
	Type             AuthoritySourceType
	StewardBindingID uuid.UUID
	ExternalRef      string
	EvidencePointer  string
	Summary          string
}

type AuthoritySourceRelations struct {
	Steward IdentityBinding
}

func NewAuthoritySource(p AuthoritySourceParams) (AuthoritySource, error) {
	id := p.SourceID
	if id == uuid.Nil {
		generated, err := newID()
		if err != nil {
			return AuthoritySource{}, err
		}
		id = generated
	}
	s := AuthoritySource{
		SourceID:         id,
		OrganizationID:   p.OrganizationID,
		Type:             p.Type,
		StewardBindingID: p.StewardBindingID,
		ExternalRef:      p.ExternalRef,
		EvidencePointer:  p.EvidencePointer,
		Summary:          p.Summary,
	}
	if err := s.Validate(); err != nil {
		return AuthoritySource{}, err
	}
	return s, nil
}

func NewAuthoritySourceWithRelations(p AuthoritySourceParams, rel AuthoritySourceRelations) (AuthoritySource, error) {
	if err := rel.Validate(p.OrganizationID, p.StewardBindingID); err != nil {
		return AuthoritySource{}, err
	}
	return NewAuthoritySource(p)
}

func (s AuthoritySource) Validate() error {
	if err := requireID("source_id", s.SourceID); err != nil {
		return err
	}
	if err := requireID("organization_id", s.OrganizationID); err != nil {
		return err
	}
	if err := requireID("steward_binding_id", s.StewardBindingID); err != nil {
		return err
	}
	if err := requireNonEmpty("external_ref", s.ExternalRef); err != nil {
		return err
	}
	if err := requireNonEmpty("summary", s.Summary); err != nil {
		return err
	}
	if !validAuthoritySourceType(s.Type) {
		return fmt.Errorf("%w: type", ErrInvalidInput)
	}
	return nil
}

func (r AuthoritySourceRelations) Validate(orgID, stewardID uuid.UUID) error {
	if err := SameOrganization(orgID, r.Steward.OrganizationID); err != nil {
		return err
	}
	if r.Steward.BindingID != stewardID {
		return fmt.Errorf("%w: steward_binding_id", ErrInvalidReference)
	}
	return r.Steward.Validate()
}

func validAuthoritySourceType(t AuthoritySourceType) bool {
	switch t {
	case SourceHumanApproval, SourceRole, SourcePolicy, SourceContract, SourceProcurementMandate, SourceBoardMandate, SourceCredential, SourceMachineAgreement, SourceOther:
		return true
	default:
		return false
	}
}
