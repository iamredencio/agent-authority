package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iamredencio/agent-authority/internal/domain"
)

func (s *Store) CreateDecision(ctx context.Context, d domain.Decision) error {
	if err := d.Validate(); err != nil {
		return err
	}
	reasons, err := json.Marshal(d.Reasons)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO decisions (
			decision_id, organization_id, mandate_id, mission_id, actor_binding_id,
			act_type, act, result, reasons, valid_until, decided_at, evidence_record_id
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11, $12
		)
	`,
		d.DecisionID,
		d.OrganizationID,
		d.MandateID,
		d.MissionID,
		d.ActorBindingID,
		d.ActType,
		[]byte(d.Act),
		d.Result,
		reasons,
		d.ValidUntil,
		d.DecidedAt,
		d.EvidenceRecordID,
	)
	return mapError(err)
}

func (s *Store) GetDecision(ctx context.Context, organizationID, decisionID uuid.UUID) (domain.Decision, error) {
	var d domain.Decision
	var act, reasons []byte
	var validUntil *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT decision_id, organization_id, mandate_id, mission_id, actor_binding_id,
			act_type, act, result, reasons, valid_until, decided_at, evidence_record_id
		FROM decisions
		WHERE organization_id = $1 AND decision_id = $2
	`, organizationID, decisionID).Scan(
		&d.DecisionID,
		&d.OrganizationID,
		&d.MandateID,
		&d.MissionID,
		&d.ActorBindingID,
		&d.ActType,
		&act,
		&d.Result,
		&reasons,
		&validUntil,
		&d.DecidedAt,
		&d.EvidenceRecordID,
	)
	if err != nil {
		return domain.Decision{}, mapError(err)
	}
	d.Act = json.RawMessage(act)
	if err := json.Unmarshal(reasons, &d.Reasons); err != nil {
		return domain.Decision{}, err
	}
	d.DecidedAt = d.DecidedAt.UTC()
	if validUntil != nil {
		t := validUntil.UTC()
		d.ValidUntil = &t
	}
	return d, nil
}
