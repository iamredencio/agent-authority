# Phase 1 verification report

**Phase:** 1  
**Title:** Core authority domain model  
**Date:** 2026-09-06  
**Result:** All P1 acceptance criteria **PASS**. Phase 1 remains `approved` and is **not** marked `locked`. Phase 2 has **not** been started.

## Commit / branch context

| Item | Value |
| --- | --- |
| Branch | `phase-1-domain-model` |
| Base commit | `a8dfce1eae89bd31c47d1bf22263c4495bcfb75e` |
| Base subject | Lock Phase 0 after attenuation and portable-mandate clarifications. |
| Working tree | Uncommitted Phase 1 implementation (no commit was requested) |
| Go module | `github.com/iamredencio/agent-authority` |
| Go version used | `go1.24.6 darwin/arm64` |

Phase 0 is `locked`. A human explicitly approved starting Phase 1 and requested this implementation. The roadmap Phase 1 state was changed from `planned` to `approved` before code was written.

## Scope

Persist the core authority aggregates and invariants in Go + PostgreSQL. No public Decision API.

In scope: Go module, `internal/domain`, PostgreSQL migrations, and a PostgreSQL repository sufficient for create/read round-trips of Organization, IdentityBinding, AuthoritySource, Mission, and Mandate.

Out of scope: Phase 2+ engines, APIs, adapters, evidence, Redis, SDKs, and portable mandate signing.

## Specification sections implemented

Phase 1 implements the **logical persisted model** for:

| Section | What was implemented |
| --- | --- |
| §3 Identity (S-ID-1, S-ID-2, S-ID-4, S-ID-5) | `IdentityBinding` stores provider + subject (+ optional attestation metadata). No local IdP, password, or proprietary global agent ID. |
| §4 Authority source (S-AS-1 field shape, S-AS-2) | `AuthoritySource` has type, identifier (`source_id` / `external_ref`), steward, and evidence pointer. |
| §5 Mission (S-MS-1, S-MS-3 as a stored reference) | Mission stores purpose, principal, intended outcome, validity window, state, and authority source. No state machine. |
| §6 Delegation (storage only) | `parent_mandate_id` and `delegation_depth` are stored. `delegation_depth` must be ≥ 0. No issuance or attenuation engine. |
| §11 Financial authority (field only) | Mandate `budget` JSONB is required as a field; empty object is allowed. No spend execution. |
| §14.1–§14.2 Portable agent mandate | All required logical fields are represented on `Mandate`. |
| §14.4 S-PM-2 / D-014 | Logical model only. No wire format and no cryptographic signing. |
| §17 S-NF-5 | Organization is the tenancy boundary. Cross-tenant relations fail closed. |

Not implemented (later phases): S-ID-3 decision path, S-AS-3/S-AS-4 engines, S-MS-2/S-MS-4/S-MS-5 evaluation, §6–§10 engines, §15 Decision API, evidence hash chain, adapters.

Architecture coverage: §6.1–§6.5 data model, §11 `internal/domain`, §12 Go + PostgreSQL + UUID + UTC.

## Acceptance criteria

| ID | Criterion | Result | Evidence |
| --- | --- | --- | --- |
| P1-1 | Aggregates in architecture §6 can be created, read and persist-round-tripped. | **PASS** | Domain constructors create Organization, IdentityBinding, AuthoritySource, Mission, and Mandate. `TestPersistenceRoundTripAllAggregates` inserts and reads each of those five Phase 1 aggregates, including optional `parent_mission_id` and `parent_mandate_id`. Architecture §6.6–§6.10 (DelegationChain, Decision, Approval, Revocation/Containment, EvidenceRecord) are later-phase entities and were not implemented, per architecture §4 and the Phase 1 roadmap. |
| P1-2 | Identity is stored only as a binding to an external reference. | **PASS** | `IdentityBinding` fields are provider, subject, optional display name and attestation metadata. `TestNewIdentityBindingExternalReference` asserts no password/secret/credential fields. `TestIdentityBindingRemainsExternalAfterRoundTrip` persists provider + subject only. |
| P1-3 | A mandate cannot be created without mission, principal, agent and authority source. | **PASS** | `NewMandate` rejects nil `mission`, `principal`, `agent`, and `authority_source`. Persistence `CreateMandate` also rejects missing or cross-tenant references (`TestPersistenceRejectsMissingMandateRefs`). Organization and issuer are also required. |
| P1-4 | Tests cover missing required fields and tenancy scoping. | **PASS** | Domain table tests for all five aggregates; `SameOrganization`; cross-tenant constructors for source, mission, and mandate; persistence tenant isolation and cross-tenant FK/application checks. |
| P1-5 | Lint and tests pass. `reports/PHASE-1-VERIFICATION-REPORT.md` exists. | **PASS** | `go test ./...` passed. `gofmt -l .` produced no files. `go vet ./...` passed. `staticcheck ./...` passed. This report exists. |
| P1-6 | No Decision API, adapter or evidence-chain implementation. | **PASS** | No `cmd/`, `api/`, `internal/decision`, `internal/policy`, `internal/evidence`, `internal/adapters`, HTTP server, OPA/Rego, MCP, OIDC adapter, or evidence hash-chain types. |

## Tests executed and results

Command: `go test ./... -count=1 -timeout 3m`

| Package | Result |
| --- | --- |
| `github.com/iamredencio/agent-authority/internal/domain` | **PASS** (0.262s) |
| `github.com/iamredencio/agent-authority/internal/postgres` | **PASS** (3.433s) |

Domain coverage includes:

- aggregate construction
- missing required fields
- invalid IDs, enums, JSON, and references
- organization / tenant isolation
- IdentityBinding as an external identity reference
- mandate required-field validation (specification §14)
- negative `delegation_depth` rejection; zero depth allowed
- parent mandate stored without running a delegation engine

## PostgreSQL integration-test results

PostgreSQL was available via Docker (`postgres:16-alpine`) started by the test helper when `TEST_DATABASE_URL` was unset.

Command: `go test ./internal/postgres/ -count=1 -v -timeout 3m`

| Test | Result |
| --- | --- |
| `TestMigrateIsIdempotent` | PASS |
| `TestPersistenceRoundTripAllAggregates` | PASS |
| `TestPersistenceTenantIsolation` | PASS |
| `TestPersistenceRejectsMissingMandateRefs` | PASS |
| `TestPersistenceRejectsNegativeDelegationDepth` | PASS |
| `TestIdentityBindingRemainsExternalAfterRoundTrip` | PASS |

Migrations are applied from embedded SQL files. Schema is not created by ad-hoc runtime `CREATE TABLE` outside the migration runner.

## Lint / static analysis

| Check | Result |
| --- | --- |
| `gofmt -l .` | PASS (no unformatted files) |
| `go vet ./...` | PASS |
| `staticcheck ./...` (`honnef.co/go/tools` v0.8.1) | PASS (no findings) |

## Files changed

Updated:

- `docs/ROADMAP.md` — Phase 1 state `approved`; current implementation phase set to Phase 1
- `README.md` — current status now records Phase 1 as approved

Added:

- `.gitignore`
- `go.mod`, `go.sum`
- `internal/domain/*.go` and `*_test.go`
- `internal/postgres/*.go` and `*_test.go`
- `internal/postgres/migrations/000001_core_authority_domain.up.sql`
- `internal/postgres/migrations/000001_core_authority_domain.down.sql`
- `reports/PHASE-1-VERIFICATION-REPORT.md`

No `cmd/`, `api/`, `deploy/`, Dockerfile, Compose file, Redis, or SDK packages were added.

## Database migrations added

| Version | File | Purpose |
| --- | --- | --- |
| `000001_core_authority_domain` | `internal/postgres/migrations/000001_core_authority_domain.up.sql` | Organizations, identity_bindings, authority_sources, missions, mandates, composite organization-scoped foreign keys, `delegation_depth >= 0` |
| `000001_core_authority_domain` | `internal/postgres/migrations/000001_core_authority_domain.down.sql` | Drop the five Phase 1 tables |

The migrator records applied versions in `schema_migrations`.

## Domain invariants implemented

- Every aggregate is organization-scoped.
- Identity is an external binding (`provider` + `subject`); Agent Authority is not an IdP.
- Authority source has type, steward, external reference, and optional evidence pointer.
- Mission stores purpose, principal, intended outcome, validity window, state, and authority source. States are represented only.
- A mandate requires organization, principal, agent, mission, authority source, issuer, §14 logical fields, and non-negative `delegation_depth`.
- Mandate JSON fields reject missing or JSON `null` values.
- Cross-tenant relations fail in constructors (when related aggregates are supplied) and in persistence (lookup + composite foreign keys).
- Organization-scoped reads of bindings, sources, missions, and mandates return `ErrNotFound` across tenants.
- Principal / agent / issuer kinds must match their mandate roles.
- Parent mission and parent mandate identifiers may be stored. No attenuation comparison and no mission transition engine.

## Explicit Phase 1 non-goals left untouched

- Delegation issuance, attenuation engine, and delegation-chain reconstruction
- Mission state machine / “right now” window evaluation as an authorization engine
- Decision API, HTTP server, OpenAPI
- OPA/Rego policy integration
- Evidence hash chain
- MCP, A2A, OIDC/Entra adapters
- Human approval workflow
- Revocation / containment engine
- Financial execution and payment adapters
- Registry federation
- SDKs
- Redis
- Docker/Kubernetes product packaging
- Portable mandate serialization or cryptographic signing
- Phase 2 scaffolding (`internal/decision`, `internal/policy`, `internal/evidence`, `internal/adapters`, `cmd/`)

## Residual risks or documentation gaps

- Mandate `scope`, communication, execution, constraint, budget, approval, and evidence JSON documents are stored opaquely. Subset / attenuation algebra remains Phase 2.
- Mission and mandate `state` values are stored without transition rules. An `approved` mission can be persisted, but Phase 1 does not refuse issuance against a non-approved mission.
- `parent_mandate_id` may be stored with equal `delegation_depth`; Phase 2 must reject that at issuance time.
- Mandate wire format and cryptographic signing profile remain unspecified (D-014).
- Architecture §11 lists `internal/domain` but not `internal/postgres`. The extra package is the Phase 1 persistence adapter requested by the roadmap; it is not a product-scope expansion.
- No production deploy path yet (expected; first deployable binary is Phase 3+).
- Integration tests start PostgreSQL with Docker when `TEST_DATABASE_URL` is unset. That is a test fixture, not product packaging.

## Phase 2 has not been started

**Phase 2 has NOT been started.** It remains `planned` and is not approved.

No mission state machine, originating-mandate issuance engine, attenuation checks, or delegation-chain reconstruction were implemented. This report does not approve Phase 2.

Phase 1 is **not** marked `locked`. Human acceptance of this report is required before that state change.

This close-out stops after Phase 1.
