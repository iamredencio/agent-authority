-- Phase 3 evidence-ready decisions. Hash chaining remains Phase 4.
-- mandate_id / mission_id / actor_binding_id are nullable so a deny can be
-- stored when authentication has no matching mandate.

CREATE TABLE decisions (
    decision_id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (organization_id),
    mandate_id UUID,
    mission_id UUID,
    actor_binding_id UUID,
    act_type TEXT NOT NULL CHECK (act_type IN (
        'communicate',
        'execute',
        'delegate',
        'spend',
        'control'
    )),
    act JSONB NOT NULL,
    result TEXT NOT NULL CHECK (result IN ('allow', 'deny', 'pending_approval')),
    reasons JSONB NOT NULL,
    valid_until TIMESTAMPTZ,
    decided_at TIMESTAMPTZ NOT NULL,
    evidence_record_id UUID,
    UNIQUE (organization_id, decision_id),
    FOREIGN KEY (organization_id, mandate_id)
        REFERENCES mandates (organization_id, mandate_id),
    FOREIGN KEY (organization_id, mission_id)
        REFERENCES missions (organization_id, mission_id),
    FOREIGN KEY (organization_id, actor_binding_id)
        REFERENCES identity_bindings (organization_id, binding_id)
);

CREATE INDEX decisions_organization_id_idx ON decisions (organization_id);
CREATE INDEX decisions_mandate_id_idx ON decisions (organization_id, mandate_id);
