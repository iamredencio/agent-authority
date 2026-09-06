# Phase 2 verification report

**Phase:** 2  
**Title:** Mission and delegation engine  
**Date:** 2026-09-06  
**Work order:** GitHub Issue #8  
**Result:** All P2 acceptance criteria **PASS**. Phase 2 remains `approved` and is **not** marked `locked`. Phase 3 has **not** been started.

## Commit / branch context

| Item | Value |
| --- | --- |
| Branch | `phase-2-mission-delegation` |
| Base | `origin/main` (`6d65d94` — Phase 1 locked) |
| Go module | `github.com/iamredencio/agent-authority` |
| Go version used | `go1.24.6 darwin/arm64` |

Phase 0 and Phase 1 are `locked`. Issue #8 records explicit human approval for Phase 2. The roadmap Phase 2 state was changed from `planned` to `approved` on this branch before engine code was treated as in-scope.

## Scope

Enforce mission lifecycle and attenuation-only delegation on the locked Phase 1 domain model. No public Decision API.

In scope: mission state machine, originating and child mandate issuance, specification §14.3 attenuation, delegation-chain reconstruction, and tests covering the Phase 2 acceptance surface.

Out of scope: Phase 3+ APIs, policy, evidence, adapters, Redis, SDKs, portable mandate signing, and human approval workflow.

## Specification sections implemented

| Section | What was implemented |
| --- | --- |
| §5 Mission (S-MS-2, S-MS-4, S-MS-5 as domain gates) | `AssertAuthorizes` and issuance refuse non-approved or out-of-window missions. Ending/suspending/expiring is modeled as a state that no longer authorizes. |
| §6 Delegation (S-DL-1–S-DL-8 except revocation cascade engine) | Child issuance requires a parent, attenuation on every §14.3 axis, strict depth decrease, window containment, and fail-closed ancestor walk. |
| §7 / §8 communication and execution (as authority sets) | Explicit target arrays; unknown or wildcard destinations/actions fail closed. Independent execution vs communication sets. |
| §11 Financial authority (attenuation only) | Budget subset against remaining parent budget, same unit, no extra category. No payment adapter. |
| §14.3 Authority axes | Deterministic subset / stricter-or-equal checks. Equality allowed except `delegation_depth`. |
| §14.4 / D-014 | Still logical model only. No wire format and no signing. |

Not implemented (later phases): Decision API, OPA/Rego, evidence hash chain, approval workflow resolution, revocation/containment engine, adapters.

## Mission transition rules

Allowed edges:

| From | To |
| --- | --- |
| `draft` | `pending_approval`, `ended` |
| `pending_approval` | `approved`, `ended` |
| `approved` | `suspended`, `ended`, `expired` |
| `suspended` | `ended`, `expired` |

`ended` and `expired` are terminal. `draft → approved` is rejected (must pass through `pending_approval`). Resume from `suspended` is not specified and is rejected. `expired` is allowed only after the inclusive expiry instant.

Approving a sub-mission requires the parent mission to be present, approved, and in window, and the child window to sit inside the parent window. There is no `rejected` mission state in the specification; non-approved tests cover `draft`, `pending_approval`, `suspended`, `ended`, and `expired`.

## Attenuation / subset semantics

A child is accepted only when it is no broader than its parent on every axis. Equality is allowed except `delegation_depth`, which must strictly decrease.

| Axis | Semantics |
| --- | --- |
| `scope` | JSON object or array permission subset. Child may drop keys/items; adding or widening fails. |
| `communication_authority` | JSON array of objects, each with a non-empty `destination` string. Child set ⊆ parent set. `*`, `**`, `any`, or `*` inside a destination fails closed. |
| `execution_authority` | Same as communication, required key `action`. |
| `constraints` | JSON object. Parent keys cannot be removed. Known tighter-or-equal vocabulary: `rate`/`max_rate`/`max_amount`/`limit` (child ≤ parent), `expiry`/`until` (child ≤ parent), `not_before`/`after` (child ≥ parent), `geo`/`data`/`residency` (string equal or set subset). Unknown keys may not be added or changed. |
| `budget` | Empty object = no spend authority. Present budget requires `unit`, `amount`, `remaining`, `categories` only. Child unit must match; child amount and remaining must be ≤ parent remaining; child categories ⊆ parent categories. Extra fields fail closed. |
| `expiry` / `not_before` | Child expiry ≤ parent expiry; child `not_before` ≥ parent `not_before`. Mandate window must also sit inside the acting mission window. |
| `delegation_depth` | Parent depth must be > 0. Child depth must be ≥ 0 and strictly smaller. |
| `approval_requirements` | JSON object. Known keys `human`/`required` (true is stricter), `count`/`min_approvals` (greater is stricter), `approvers`/`requirements` (superset is stricter). Unknown keys may not be added or changed. |
| `evidence_requirements` | JSON array of strings: child must be a superset. Object form uses the same stricter-or-equal rules. Type mismatch fails closed. |
| `mission` | Same mission as the parent, or a recorded sub-mission whose `parent_mission_id` is the parent mandate’s mission, approved and in window, and unable to outlive that parent. |
| `authority_source` | Same `source_id` only. No source-widening lattice is defined, so a different source cannot be proven not to widen and is rejected. |

Malformed or unknown structured authority fails closed (`ErrMalformedAuthority` or `ErrAmplification`). Phase 1 constructors (`NewMandate`, `CreateMandate`) still persist records without running issuance/attenuation.

## Acceptance criteria

| ID | Criterion | Result | Evidence |
| --- | --- | --- | --- |
| P2-1 | Actions as domain operations refuse a non-approved or out-of-window mission. | **PASS** | `AssertAuthorizes`, `IssueOriginatingMandate`, and `IssueChildMandate` reject draft/pending/suspended/ended/expired and times outside `[not_before, expiry]`. Tests: `TestAssertAuthorizes`, `TestIssueOriginatingMandateRejectsNonApprovedMissions`, `TestIssueOriginatingMandateRejectsOutOfWindow`, `TestIssueOriginatingMandateRejectsDraftMission`. |
| P2-2 | Child issuance fails if any attenuation axis is widened. Equal-on-axis children pass where §14.3 allows equality. | **PASS** | `TestIssueChildEqualOnAxis`, `TestIssueChildNarrowerAuthority`, `TestIssueChildRejectsEachWidenedAxis` (scope, communication, execution, constraints, budget, time, approval, evidence, authority source), `TestIssueChildRejectsAmplificationPersistence`. |
| P2-3 | `delegation_depth` 0 cannot issue children; child depth is strictly smaller. Other axes are not required to strictly decrease. | **PASS** | `TestParentDepthZeroCannotDelegate`, equal-on-axis child only decreases depth, `TestIssueChildRejectsEachWidenedAxis/delegation_depth_equal`. |
| P2-4 | Child expiry / `not_before` cannot exceed parent windows. | **PASS** | `TestChildTimeWindowInsideParent`, `TestChildTimeWindowOutsideParentFails`, widening cases for expiry and `not_before`. |
| P2-5 | Ancestor walk fails closed on a missing link. | **PASS** | `ReconstructDelegationChain` fails on missing, cross-tenant, revoked, or cyclic ancestors. Tests: `TestReconstructDelegationChainMissingLinkFailsClosed`, `TestReconstructDelegationChainCrossTenantFailsClosed`, `TestReconstructDelegationChainRevokedAncestorFailsClosed`, `TestReconstructDelegationChainCycleFailsClosed`, `TestReconstructChainMissingLinkPersistence`. Valid chain: `TestReconstructDelegationChainSuccess`, `TestIssueChildMandateAndChainPersistence`. |
| P2-6 | Lint/tests pass and `reports/PHASE-2-VERIFICATION-REPORT.md` exists. No Phase 3 API. | **PASS** | `go test ./...`, `gofmt`, `go vet`, and `staticcheck` passed. This report exists. Packages remain `internal/domain` and `internal/postgres` only. No `cmd/`, `api/`, `internal/decision`, `internal/policy`, `internal/evidence`, or `internal/adapters`. |

## Tests executed and results

Command: `go test ./... -count=1 -timeout 3m`

| Package | Result |
| --- | --- |
| `github.com/iamredencio/agent-authority/internal/domain` | **PASS** |
| `github.com/iamredencio/agent-authority/internal/postgres` | **PASS** |

Phase 1 regression tests remain green, including persistence round-trip, tenant isolation, and constructor-only parent storage without attenuation.

## PostgreSQL integration-test results

PostgreSQL was available via Docker (`postgres:16-alpine`) started by the existing test helper when `TEST_DATABASE_URL` was unset.

| Test | Result |
| --- | --- |
| `TestTransitionMissionPersists` | PASS |
| `TestTransitionMissionRejectsInvalidEdge` | PASS |
| `TestIssueOriginatingMandatePersistence` | PASS |
| `TestIssueOriginatingMandateRejectsDraftMission` | PASS |
| `TestIssueChildMandateAndChainPersistence` | PASS |
| `TestIssueChildRejectsAmplificationPersistence` | PASS |
| `TestReconstructChainMissingLinkPersistence` | PASS |
| `TestCreateSubMissionRejectsOutlivingParent` | PASS |
| Phase 1 persistence suite | PASS |

## Lint / static analysis

| Check | Result |
| --- | --- |
| `gofmt -l .` | PASS (no unformatted files) |
| `go vet ./...` | PASS |
| `staticcheck ./...` (`honnef.co/go/tools` v0.8.1) | PASS (no findings) |

## Files changed

Updated:

- `docs/ROADMAP.md` — Phase 2 state `approved`; current implementation phase set to Phase 2; Phase 0 and Phase 1 remain `locked`
- `README.md` — current status now records Phase 1 locked and Phase 2 approved
- `internal/domain/errors.go` — Phase 2 sentinel errors
- `internal/domain/mission.go` — window helpers, authorize gate, sub-mission window check
- `internal/domain/mandate.go` — comment: issuance is Phase 2
- `internal/domain/fixtures_test.go` — shared `testNow`
- `internal/postgres/missions.go` — parent-window check, `UpdateMission`, `TransitionMission`

Added:

- `internal/domain/mission_lifecycle.go` and `mission_lifecycle_test.go`
- `internal/domain/authority.go`
- `internal/domain/attenuation.go`
- `internal/domain/issuance.go` and `issuance_test.go`
- `internal/domain/chain.go` and `chain_test.go`
- `internal/postgres/issuance.go`
- `internal/postgres/engine_test.go`
- `reports/PHASE-2-VERIFICATION-REPORT.md`

No `cmd/`, `api/`, `deploy/`, Dockerfile, Redis, SDK, Decision API, OpenAPI, OPA/Rego, or evidence-chain packages were added.

## Database migrations

None. Phase 2 uses the locked Phase 1 schema. Persistence additions are `UPDATE` of mission state and domain-gated issuance on existing tables.

## Explicit Phase 2 non-goals left untouched

- HTTP/JSON Decision API, HTTP server, OpenAPI
- OPA/Rego policy evaluation
- Human approval workflow/API (Phase 2 only transitions `pending_approval`; it does not resolve approvers)
- Evidence hash chain / evidence plane
- MCP, A2A, OIDC/Entra/Okta/CrowdStrike/SPIFFE adapters
- Revocation or containment engine
- Financial execution / payment adapters
- Registry federation / counterparty trust
- SDKs
- Redis
- Portable mandate wire format or cryptographic signing
- Phase 3 scaffolding (`internal/decision`, `internal/policy`, `internal/evidence`, `internal/adapters`, `cmd/`, `api/`)

## Residual risks or documentation gaps

- Constraint tightness for unknown keys is fail-closed equality only. A later decision may define a richer constraint vocabulary.
- Authority-source attenuation is same-`source_id` only. The specification allows a non-widening different source, but no lattice exists; a later decision is required before accepting a different source.
- `NewMandate` / `CreateMandate` remain storage constructors and can still persist a non-attenuated parent/child pair. Authority use and issuance must go through `IssueOriginatingMandate`, `IssueChildMandate`, and `AssertMandateUsable`.
- Resume from `suspended` is not implemented because it is not specified.
- There is no specification state named `rejected`. Issue #8 mentioned that word in test expectations; non-approved coverage uses the documented states instead.
- Mandate wire format and cryptographic signing profile remain unspecified (D-014).
- Integration tests start PostgreSQL with Docker when `TEST_DATABASE_URL` is unset. That is a test fixture, not product packaging.

## Phase 3 has not been started

**Phase 3 has NOT been started.** It remains `planned` and is not approved.

No Decision API, HTTP server, OpenAPI, OPA/Rego policy engine, or OpenTelemetry PDP instrumentation was implemented. This report does not approve Phase 3.

Phase 2 is **not** marked `locked`. Human acceptance of this report is required before that state change.

This close-out stops after Phase 2.
