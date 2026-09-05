# Architecture

**Status:** Architecture and data model  
**Product:** Agent Authority  
**Normative parent:** [SPECIFICATION.md](SPECIFICATION.md)

This document describes how the specification is realized. It does not add product scope. Implementation is allowed only for the currently approved roadmap phase.

---

## 1. Principle

Embrace existing standards and vendors; own the **authority enforcement** and **evidence** layer.

```
                    untrusted enforcement
         ┌──────────────────────────────────────┐
         │ runtimes │ MCP │ A2A │ API gateways  │
         └──────────────┬───────────────────────┘
                        │ decision request
                        ▼
         ┌──────────────────────────────────────┐
         │         Authority plane              │
         │  mission · mandate · policy · PDP    │
         └───────┬──────────────────┬───────────┘
                 │                  │
                 ▼                  ▼
         ┌──────────────┐   ┌───────────────────┐
         │ Control plane│   │ Evidence plane    │
         │ revoke/      │   │ hash-chained log  │
         │ contain      │   │ independent SoT   │
         └──────────────┘   └───────────────────┘
                 │
                 ▼
         adapters: IdP · payments · registries · network
```

The authority plane is the trusted decision point. The evidence plane is the independently verifiable system of record for what was intended, decided, attempted and revoked. Everything that executes tools or forwards protocol traffic is an **untrusted enforcement environment**.

---

## 2. System context

**Inside the product**

- Organization/tenant boundary
- Identity bindings (not an IdP)
- Missions and authority sources
- Mandate issuance and attenuated delegation
- Policy decision point (PDP)
- Approval records (workflow arrives in Phase 7)
- Revocation and containment (Phase 8)
- Evidence log
- Adapter ports

**Outside the product**

- Entra ID, Okta, SPIFFE/SPIRE, CrowdStrike, custom IdPs, workload identity
- Agent runtimes and frameworks
- MCP servers and MCP gateways
- A2A endpoints
- REST/OpenAPI services
- Enterprise policy systems (input to, or implementation of, the PDP)
- Payment rails (Stripe, card networks, SEPA, x402, …)
- Cloud / community agent registries
- WORM/object-lock object stores (optional evidence backend later)

---

## 3. Planes

### 3.1 Authority plane

Owns:

- mandate lifecycle
- mission state
- attenuation checks
- policy evaluation
- decision API

Does **not** own: model inference, tool execution, packet forwarding, payment settlement.

### 3.2 Evidence plane

Owns:

- append-only, hash-chained records
- later: signing and WORM backends

Must remain writable and readable even when a runtime is compromised, paused or lying.

### 3.3 Control plane (out of band)

Owns:

- revocation of mandates, derived credentials and missions
- containment instructions to adapters
- does not require a successful runtime callback to take effect

Authority-plane decisions consult revocation state **before** returning `allow`.

---

## 4. Logical components

| Component | Responsibility | First phase |
| --- | --- | --- |
| Identity binding service | Store and resolve external identity references | 1 (model), 6 (OIDC/Entra adapter) |
| Authority source registry | Traceable source records | 1 |
| Mission service | Mission lifecycle and approval state | 1 (model), 2 (engine), 7 (human workflow) |
| Mandate service | Issue originating mandates | 1 (model), 2 (engine) |
| Delegation engine | Attenuation-only child issuance | 2 |
| Policy engine | OPA/Rego-compatible evaluation | 3 |
| Decision API | Independent authz for each act | 3 |
| Evidence log | Hash-chained append-only store | 4 (hooks designed in 1–3) |
| MCP adapter | Ask Decision API for MCP-bound acts | 5 |
| OIDC/Entra adapter | Inbound identity verification | 6 |
| Approval workflow | Human/external approvals | 7 |
| Revocation/containment | Out-of-band control | 8 |
| Financial authority | Spend decisions; rail ports | 9 |
| Federation / attestations | External registries, KYA hooks | 10 |

Components listed for a future phase MUST NOT be implemented early except as inert fields required by the current phase’s data model.

---

## 5. Trust boundaries

| Zone | Trust |
| --- | --- |
| Authority + control planes | Trusted computing base for decisions and revocation |
| Evidence plane | Trusted for record integrity; must detect tampering via the hash chain |
| PostgreSQL | Trusted store in early phases; deploy with access control and backups |
| Adapters | Semi-trusted: they may fail or be slow; they must not mint authority |
| Agent runtime, MCP, A2A, API gateways | **Untrusted.** Inputs are claims. Outputs are side effects to be evidenced. |
| Payment rails | Untrusted for authorization; trusted only as execution adapters after `allow` |
| External registries | Untrusted directories; attestations are claims until Phase 10 verifies them |

**Authn ≠ authz.** A valid OIDC token or gateway session is an identity claim. Execution still requires a mandate decision.

---

## 6. Data model

This is the logical model for Phase 1+. Physical DDL is produced in Phase 1, not here as application code.

### 6.1 Organization

Tenancy boundary.

- `organization_id` (UUID)
- `name`
- `created_at`

All other aggregates are organization-scoped.

### 6.2 IdentityBinding

- `binding_id` (UUID)
- `organization_id`
- `kind` (`principal` | `agent` | `issuer`)
- `provider` (`entra` | `okta` | `spiffe` | `crowdstrike` | `workload` | `oidc` | `custom`)
- `subject` (provider-native identifier)
- `display_name` (optional)
- `attestation_meta` (JSONB, optional)
- `status` (`active` | `disabled`)

The product does not mint users or workloads. It binds to them.

### 6.3 AuthoritySource

- `source_id` (UUID)
- `organization_id`
- `type` (`human_approval` | `role` | `policy` | `contract` | `procurement_mandate` | `board_mandate` | `credential` | `machine_agreement` | `other`)
- `steward_binding_id`
- `external_ref` (URI or vendor identifier)
- `evidence_pointer` (optional)
- `summary`

### 6.4 Mission

- `mission_id` (UUID)
- `organization_id`
- `principal_binding_id`
- `purpose`
- `intended_outcome`
- `parent_mission_id` (optional; sub-mission)
- `not_before`, `expiry`
- `state` (`draft` | `pending_approval` | `approved` | `suspended` | `ended` | `expired`)
- `authority_source_id`

An originating mandate may be issued only for `approved` missions inside their window.

### 6.5 Mandate (portable agent mandate)

Logical fields — specification §14.

- `mandate_id`, `version`, `organization_id`
- `principal_binding_id`, `agent_binding_id`
- `mission_id`, `authority_source_id`
- `parent_mandate_id` (null if originating)
- `scope` (JSONB; bounded actions/resources taxonomy)
- `communication_authority` (JSONB; explicit targets)
- `execution_authority` (JSONB; explicit actions)
- `constraints` (JSONB)
- `budget` (JSONB; unit, amount, remaining, categories)
- `not_before`, `expiry`, `issued_at`
- `delegation_depth` (integer ≥ 0)
- `approval_requirements` (JSONB)
- `evidence_requirements` (JSONB)
- `issuer_binding_id`
- `state` (`active` | `revoked` | `expired` | `superseded`)

Serialization (Phase 2/5 boundary): JSON object matching these fields. Cryptographic signing of the serialized mandate is required before an external adapter treats a mandate as portable (see D-014).

### 6.6 DelegationChain

Materialized for decision and evidence:

- ordered list of `mandate_id` from origin to actor
- stored on the child and/or reconstructed by parent walk
- reconstruction MUST fail closed if a link is missing or an ancestor is revoked

### 6.7 Decision

- `decision_id` (UUID)
- `organization_id`
- `mandate_id`, `mission_id`
- `actor_binding_id`
- `act_type` (`communicate` | `execute` | `delegate` | `spend` | `control`)
- `act` (JSONB; destination and/or action)
- `result` (`allow` | `deny` | `pending_approval`)
- `reasons` (stable machine codes + human text)
- `valid_until` (optional short TTL for the decision, not the mandate)
- `decided_at`
- `evidence_record_id` (set once the evidence plane exists)

### 6.8 Approval

- `approval_id`
- `organization_id`
- `subject_type` (`mission` | `mandate` | `decision`)
- `subject_id`
- `requirement_ref`
- `approver_binding_id`
- `result` (`granted` | `denied`)
- `decided_at`

Phase 7 implements workflow. Earlier phases may persist the requirement on the mandate without a workflow engine.

### 6.9 Revocation / Containment

- `control_event_id`
- `organization_id`
- `action` (`revoke_mandate` | `revoke_mission` | `revoke_adapter_credential` | `revoke_payment_authority` | `contain_network` | `contain_children`)
- `subject_id`
- `cascade` (boolean; default true for mandates)
- `actor_binding_id`
- `reason`
- `effective_at`

Decisions consult an authoritative revocation view, not adapter acknowledgement.

### 6.10 EvidenceRecord

- `record_id` (UUID, time-ordered preferred)
- `organization_id`
- `seq` (monotonic per chain)
- `type` (see §7)
- `occurred_at`
- `payload` (JSONB)
- `payload_sha256`
- `prev_sha256`
- `record_sha256` (hash over canonical payload + prev + metadata)
- `signature` (nullable until signing lands)
- `schema_version`

One hash chain per organization in the initial design. Cross-tenant records MUST NOT share a chain.

---

## 7. Evidence record types

Minimum types (specification §9):

| Type | When |
| --- | --- |
| `mission.created` / `mission.approved` / `mission.ended` | Mission lifecycle |
| `mandate.issued` | Originating mandate |
| `mandate.delegated` | Child mandate |
| `authority.decision` | Every PDP result |
| `approval.granted` / `approval.denied` | Approval acts |
| `call.tool` / `call.api` / `call.payment` | Authorized or attempted calls |
| `result.observed` / `side_effect.observed` | Outcomes (runtime claim + plane receipt) |
| `revocation.applied` | Revocation |
| `containment.applied` | Containment |

Hash algorithm for Phase 4: SHA-256. Canonicalization: UTF-8 JSON with sorted keys and no insignificant whitespace (locked in Phase 4 tests).

---

## 8. Decision flow

```
adapter / runtime
    → authenticate caller (claim)
    → Decision API
        → resolve identity binding
        → load mandate + reconstruct chain
        → reject if revoked / expired / mission not approved
        → match communication and/or execution authority
        → evaluate constraints + budget
        → evaluate policy (OPA/Rego-compatible)
        → evaluate approval requirements
        → fail closed on any gap
        → persist Decision
        → append EvidenceRecord (from Phase 4)
    → return allow | deny | pending_approval
```

Adapters enforce locally only by obeying this result. They never infer allow from authentication.

---

## 9. Policy engine

- Initial language compatibility: **OPA/Rego**.
- Embedded vs sidecar is deferred to a Phase 3 decision (see D-006).
- Policy may deny what a mandate appears to allow. Policy MUST NOT allow what a mandate does not grant (no policy-based amplification).
- Inputs to policy: organization, mandate (minus secrets), mission, act, identity claims, time.

---

## 10. Adapter model

Ports (interfaces) live in the core. Adapters implement ports.

| Port | Direction | Examples |
| --- | --- | --- |
| Identity verification | inbound | OIDC, Entra, Okta, SPIFFE |
| Decision client | outbound from untrusted env | MCP adapter, A2A shim, REST sidecar |
| Payment execution | outbound after allow | Stripe, SEPA, x402, … |
| Registry attestation | inbound | cloud agent registries |
| Revocation fan-out | outbound | MCP credentials, network policy, payment hold |

Core domain MUST compile and test without any specific vendor SDK. Vendor SDKs belong in adapter packages introduced in their phases.

---

## 11. Intended repository layout

Create packages only when the approved phase needs them:

```
cmd/                  # Go entrypoints (Phase 3+)
internal/domain/      # entities, attenuation, invariants (Phase 1–2)
internal/policy/      # Rego-compatible PDP (Phase 3)
internal/decision/    # Decision API (Phase 3)
internal/evidence/    # hash chain (Phase 4)
internal/adapters/    # MCP, OIDC, payments, registries (Phase 5+)
api/                  # OpenAPI or proto for the Decision API (Phase 3)
deploy/               # Docker, later Helm/K8s manifests
docs/
reports/
```

No application code exists in Phase 0.

---

## 12. Stack

| Concern | Choice | Notes |
| --- | --- | --- |
| Language | Go | Single backend language until an SDK phase |
| Database | PostgreSQL | Domain + initial evidence log |
| Policy | OPA/Rego-compatible | Amplification forbidden |
| Redis | Absent | Add only with a justifying decision |
| Telemetry | OpenTelemetry | Traces, metrics, logs around the PDP |
| Packaging | Docker | Kubernetes-ready from first deployable binary |
| Time | UTC, RFC 3339 | Decision “right now” uses server time |
| IDs | UUID | Prefer time-ordered UUIDs when available |

---

## 13. Fail-closed rules (implementation invariant)

Deny when any of the following is true:

- identity binding missing or disabled
- mission missing, not approved, or outside window
- mandate missing, revoked, expired, or not yet valid
- delegation chain broken or ancestor revoked
- destination or action not positively listed
- constraint violated
- budget insufficient
- required approval absent
- revocation view unavailable
- policy evaluation errors
- evidence append required by the mandate but the evidence plane is unavailable (from Phase 4 onward)

---

## 14. Assumptions recorded here

These are implementation assumptions, not silent product expansions. They also appear in [DECISIONS.md](DECISIONS.md).

1. Multi-tenant `organization` is the isolation boundary.
2. PostgreSQL is the first evidence backend; WORM is a storage adapter later.
3. One evidence hash chain per organization.
4. Mandate interchange is JSON; signing lands before the first external adapter depends on portability.
5. No Redis in Phases 1–4.
6. Decision API is HTTP/JSON (REST) in Phase 3.
7. Human UI is out of scope until explicitly added; Phase 7 is workflow/API first.
8. Counterparty/KYA is an extension point only until Phase 10.
9. “European AgentOS” is a codename; the shipped product name is Agent Authority.
10. Sub-missions may exist but cannot outlive or outrank their parent mission.
