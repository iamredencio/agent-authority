package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) CreateMandate(ctx context.Context, m domain.Mandate) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := s.assertMandateRefs(ctx, m); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO mandates (
			mandate_id, version, organization_id, principal_binding_id, agent_binding_id,
			mission_id, authority_source_id, parent_mandate_id, scope, communication_authority,
			execution_authority, constraints, budget, not_before, expiry, issued_at,
			delegation_depth, approval_requirements, evidence_requirements, issuer_binding_id, state
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21
		)
	`,
		m.MandateID,
		m.Version,
		m.OrganizationID,
		m.PrincipalBindingID,
		m.AgentBindingID,
		m.MissionID,
		m.AuthoritySourceID,
		m.ParentMandateID,
		[]byte(m.Scope),
		[]byte(m.CommunicationAuthority),
		[]byte(m.ExecutionAuthority),
		[]byte(m.Constraints),
		[]byte(m.Budget),
		m.NotBefore,
		m.Expiry,
		m.IssuedAt,
		m.DelegationDepth,
		[]byte(m.ApprovalRequirements),
		[]byte(m.EvidenceRequirements),
		m.IssuerBindingID,
		m.State,
	)
	return mapError(err)
}

func (s *Store) GetMandate(ctx context.Context, organizationID, mandateID uuid.UUID) (domain.Mandate, error) {
	var m domain.Mandate
	var scope, comm, exec, constraints, budget, approval, evidence []byte
	err := s.pool.QueryRow(ctx, `
		SELECT mandate_id, version, organization_id, principal_binding_id, agent_binding_id,
			mission_id, authority_source_id, parent_mandate_id, scope, communication_authority,
			execution_authority, constraints, budget, not_before, expiry, issued_at,
			delegation_depth, approval_requirements, evidence_requirements, issuer_binding_id, state
		FROM mandates
		WHERE organization_id = $1 AND mandate_id = $2
	`, organizationID, mandateID).Scan(
		&m.MandateID,
		&m.Version,
		&m.OrganizationID,
		&m.PrincipalBindingID,
		&m.AgentBindingID,
		&m.MissionID,
		&m.AuthoritySourceID,
		&m.ParentMandateID,
		&scope,
		&comm,
		&exec,
		&constraints,
		&budget,
		&m.NotBefore,
		&m.Expiry,
		&m.IssuedAt,
		&m.DelegationDepth,
		&approval,
		&evidence,
		&m.IssuerBindingID,
		&m.State,
	)
	if err != nil {
		return domain.Mandate{}, mapError(err)
	}
	m.Scope = json.RawMessage(scope)
	m.CommunicationAuthority = json.RawMessage(comm)
	m.ExecutionAuthority = json.RawMessage(exec)
	m.Constraints = json.RawMessage(constraints)
	m.Budget = json.RawMessage(budget)
	m.ApprovalRequirements = json.RawMessage(approval)
	m.EvidenceRequirements = json.RawMessage(evidence)
	m.NotBefore = m.NotBefore.UTC()
	m.Expiry = m.Expiry.UTC()
	m.IssuedAt = m.IssuedAt.UTC()
	return m, nil
}

func (s *Store) assertMandateRefs(ctx context.Context, m domain.Mandate) error {
	principalOrg, principalKind, err := s.bindingOrganization(ctx, m.PrincipalBindingID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, principalOrg, "principal"); err != nil {
		return err
	}
	if principalKind != domain.KindPrincipal {
		return domain.ErrInvalidReference
	}

	agentOrg, agentKind, err := s.bindingOrganization(ctx, m.AgentBindingID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, agentOrg, "agent"); err != nil {
		return err
	}
	if agentKind != domain.KindAgent {
		return domain.ErrInvalidReference
	}

	issuerOrg, issuerKind, err := s.bindingOrganization(ctx, m.IssuerBindingID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, issuerOrg, "issuer"); err != nil {
		return err
	}
	if issuerKind != domain.KindIssuer {
		return domain.ErrInvalidReference
	}

	missionOrg, err := s.missionOrganization(ctx, m.MissionID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, missionOrg, "mission"); err != nil {
		return err
	}

	sourceOrg, err := s.sourceOrganization(ctx, m.AuthoritySourceID)
	if err != nil {
		return err
	}
	if err := rejectCrossTenant(m.OrganizationID, sourceOrg, "authority_source"); err != nil {
		return err
	}

	if m.ParentMandateID != nil {
		parentOrg, err := s.mandateOrganization(ctx, *m.ParentMandateID)
		if err != nil {
			return err
		}
		if err := rejectCrossTenant(m.OrganizationID, parentOrg, "parent_mandate_id"); err != nil {
			return err
		}
	}
	return nil
}
