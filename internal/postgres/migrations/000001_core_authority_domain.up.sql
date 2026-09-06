-- Phase 1 logical model for core authority aggregates.
-- Internal persistence only. This is not a portable mandate wire format.

CREATE TABLE organizations (
    organization_id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE identity_bindings (
    binding_id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (organization_id),
    kind TEXT NOT NULL CHECK (kind IN ('principal', 'agent', 'issuer')),
    provider TEXT NOT NULL CHECK (provider IN ('entra', 'okta', 'spiffe', 'crowdstrike', 'workload', 'oidc', 'custom')),
    subject TEXT NOT NULL CHECK (length(btrim(subject)) > 0),
    display_name TEXT,
    attestation_meta JSONB,
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
    UNIQUE (organization_id, binding_id)
);

CREATE INDEX identity_bindings_organization_id_idx ON identity_bindings (organization_id);

CREATE TABLE authority_sources (
    source_id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (organization_id),
    type TEXT NOT NULL CHECK (type IN (
        'human_approval',
        'role',
        'policy',
        'contract',
        'procurement_mandate',
        'board_mandate',
        'credential',
        'machine_agreement',
        'other'
    )),
    steward_binding_id UUID NOT NULL,
    external_ref TEXT NOT NULL CHECK (length(btrim(external_ref)) > 0),
    evidence_pointer TEXT,
    summary TEXT NOT NULL CHECK (length(btrim(summary)) > 0),
    UNIQUE (organization_id, source_id),
    FOREIGN KEY (organization_id, steward_binding_id)
        REFERENCES identity_bindings (organization_id, binding_id)
);

CREATE INDEX authority_sources_organization_id_idx ON authority_sources (organization_id);

CREATE TABLE missions (
    mission_id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (organization_id),
    principal_binding_id UUID NOT NULL,
    purpose TEXT NOT NULL CHECK (length(btrim(purpose)) > 0),
    intended_outcome TEXT NOT NULL CHECK (length(btrim(intended_outcome)) > 0),
    parent_mission_id UUID,
    not_before TIMESTAMPTZ NOT NULL,
    expiry TIMESTAMPTZ NOT NULL,
    state TEXT NOT NULL CHECK (state IN (
        'draft',
        'pending_approval',
        'approved',
        'suspended',
        'ended',
        'expired'
    )),
    authority_source_id UUID NOT NULL,
    UNIQUE (organization_id, mission_id),
    CHECK (expiry >= not_before),
    FOREIGN KEY (organization_id, principal_binding_id)
        REFERENCES identity_bindings (organization_id, binding_id),
    FOREIGN KEY (organization_id, authority_source_id)
        REFERENCES authority_sources (organization_id, source_id),
    FOREIGN KEY (organization_id, parent_mission_id)
        REFERENCES missions (organization_id, mission_id)
);

CREATE INDEX missions_organization_id_idx ON missions (organization_id);

CREATE TABLE mandates (
    mandate_id UUID PRIMARY KEY,
    version TEXT NOT NULL CHECK (length(btrim(version)) > 0),
    organization_id UUID NOT NULL REFERENCES organizations (organization_id),
    principal_binding_id UUID NOT NULL,
    agent_binding_id UUID NOT NULL,
    mission_id UUID NOT NULL,
    authority_source_id UUID NOT NULL,
    parent_mandate_id UUID,
    scope JSONB NOT NULL,
    communication_authority JSONB NOT NULL,
    execution_authority JSONB NOT NULL,
    constraints JSONB NOT NULL,
    budget JSONB NOT NULL,
    not_before TIMESTAMPTZ NOT NULL,
    expiry TIMESTAMPTZ NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    delegation_depth INTEGER NOT NULL CHECK (delegation_depth >= 0),
    approval_requirements JSONB NOT NULL,
    evidence_requirements JSONB NOT NULL,
    issuer_binding_id UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('active', 'revoked', 'expired', 'superseded')),
    UNIQUE (organization_id, mandate_id),
    CHECK (expiry >= not_before),
    FOREIGN KEY (organization_id, principal_binding_id)
        REFERENCES identity_bindings (organization_id, binding_id),
    FOREIGN KEY (organization_id, agent_binding_id)
        REFERENCES identity_bindings (organization_id, binding_id),
    FOREIGN KEY (organization_id, issuer_binding_id)
        REFERENCES identity_bindings (organization_id, binding_id),
    FOREIGN KEY (organization_id, mission_id)
        REFERENCES missions (organization_id, mission_id),
    FOREIGN KEY (organization_id, authority_source_id)
        REFERENCES authority_sources (organization_id, source_id),
    FOREIGN KEY (organization_id, parent_mandate_id)
        REFERENCES mandates (organization_id, mandate_id)
);

CREATE INDEX mandates_organization_id_idx ON mandates (organization_id);
