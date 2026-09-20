# Phase 3 verification report

**Phase:** 3  
**Title:** Policy decision API  
**Date:** 2026-09-20  
**Work order:** GitHub Issue #11  
**Result:** All P3 acceptance criteria **PASS**. Phase 3 remains `approved` and is **not** marked `locked`. Phase 4 has **not** been started.

## Commit / branch context

| Item | Value |
| --- | --- |
| Branch | `phase-3-decision-api` |
| Base | `origin/main` (`713610e` — Phase 2 locked) |
| Implementation commit | `f298ac6daaa50d3785e7e3ea531374d1227ae511` |
| Review fix | `0eca6f6c291fce514b9a0b3e5382b963edd6a9ad` — unknown organization is a request error, not an unpersisted decision |
| Go module | `github.com/iamredencio/agent-authority` |
| Go version used | `go1.24.6 darwin/arm64` |

Phase 0, Phase 1, and Phase 2 are `locked`. Issue #11 records explicit human approval to start Phase 3 after governance PR #10. The roadmap Phase 3 state was changed from `planned` to `approved` on this branch before Decision API code was treated as in-scope.

D-006 required a Phase 3 embedding choice. **D-021** records in-process OPA (official Go library). No sidecar and no Redis.

## Scope

Independent authorization for each requested act. No Phase 4+ evidence chain, adapters, or approval workflow.

In scope: HTTP/JSON Decision API, OPA/Rego-compatible policy evaluation, `allow` / `deny` / `pending_approval`, authentication treated as a claim, OpenAPI under `api/`, OpenTelemetry spans on the PDP path, and evidence-ready decision persistence.

Out of scope: MCP/A2A adapters, hash-chained evidence, IdP integrations beyond test fixtures, human approval workflow, revocation/containment engine, payment rails, federation/KYA, SDKs, Redis, and portable mandate signing.

## Specification sections implemented

| Section | What was implemented |
| --- | --- |
| §3 Identity (S-ID-3) | Identity is resolved as a claim. A valid binding without a matching mandate is `deny`. |
| §5 Mission / §6 Delegation | Decision path loads the mission, verifies the delegation chain, and refuses non-usable mandates. |
| §7 Communication (S-CA-2) | Unknown destinations fail closed. |
| §8 Execution (S-EA-1–S-EA-3) | Each act produces a distinct stored decision. Authn ≠ authz. Allow binds mission, principal/agent, mandate, action, and time. |
| §11 Financial authority (S-FA-2, S-FA-5 as decision checks) | `spend` is independently decided against remaining budget. No payment adapter. |
| §14.3 / D-004 | Policy may further deny; it cannot grant a mandate-forbidden act. |
| §15 Decision semantics (S-DC-1–S-DC-3) | Default deny. Decision API is the enforcement interface. |
| D-014 | Still logical model only. No wire format and no signing. |

Not implemented (later phases): hash-chained evidence plane, MCP adapter, OIDC/Entra verification, approval workflow resolution, revocation/containment engine, payment execution.

## Decision evaluation order

1. Establish the tenant. A missing or unknown `organization_id` is a request error (`400`), not a stored decision. The organization FK is unchanged.
2. Resolve the identity claim (binding id or provider+subject).
3. Reject if there is no matching mandate, the actor is not the mandated agent, or the binding is disabled.
4. Load mission and `VerifyDelegationChain`.
5. `AssertMandateUsable` (active, in window, approved mission).
6. Match communication / execution / spend / delegate rules. Unknown destination or action fails closed.
7. Evaluate constraints. Unmet set constraints fail closed.
8. If approval requirements are still unmet, the candidate result is `pending_approval`.
9. Evaluate policy. Policy may deny a mandate-permitted act and cannot override a prior deny.
10. Persist the decision. `evidence_record_id` stays unset.

Revocation is the stored mandate `state`. A store error while loading mandate/chain is treated as revocation-view unavailable and denied.

## Acceptance criteria

| ID | Criterion | Result | Evidence |
| --- | --- | --- | --- |
| P3-1 | Each communicate/execute/delegate/spend request produces a stored decision. | **PASS** | `TestEvaluateStoresEachActType` stores an `allow` for all four act types. `TestDecisionPersistenceRoundTrip` and `TestDecisionAPIPersistsDenyWithoutMandate` persist through PostgreSQL. A missing or unknown tenant is **not** a completed decision: `TestUnknownOrganizationIsNotADecision`, `TestHTTPUnknownOrganization`, `TestDecisionAPIUnknownOrganizationIsNotStored` return a `400` request error and store nothing. The organization FK is unchanged. |
| P3-2 | Valid authentication without a matching mandate is `deny`. | **PASS** | `TestValidAuthWithoutMandateIsDeny`, `TestFailClosedCases/unknown_mandate`, `TestDecisionAPIPersistsDenyWithoutMandate`. Reason `AUTHN_NOT_AUTHZ`. |
| P3-3 | Unknown destination or action is `deny` (fail closed). | **PASS** | `TestUnknownDestinationAndActionDeny`, `TestMatchCommunicationUnknownFailsClosed`, wildcard cases fail closed. |
| P3-4 | Policy can deny a mandate-permitted act; policy cannot allow a mandate-forbidden act. | **PASS** | `TestPolicyDeniesMandatePermittedAct`, `TestEmbeddedRegoPolicyDeny`, `TestPolicyCannotAllowMandateForbiddenAct` (allow-all policy still cannot grant `wire_funds`). |
| P3-5 | OpenTelemetry spans exist for decision evaluation. | **PASS** | `TestOpenTelemetrySpanOnEvaluate` records `authority.decision.evaluate` with `decision.result`. `cmd/decisiond` installs a tracer provider. |
| P3-6 | Lint, tests, `reports/PHASE-3-VERIFICATION-REPORT.md`. No MCP adapter. | **PASS** | `go test ./...`, `gofmt`, `go vet`, and `staticcheck` passed. This report exists. No `internal/adapters` or MCP package. |

Regression coverage also includes malformed JSON (`TestHTTPMalformedJSON`), unknown organization (`TestUnknownOrganizationIsNotADecision`, `TestHTTPUnknownOrganization`, `TestDecisionAPIUnknownOrganizationIsNotStored`), actor mismatch, broken chain, policy evaluation error, expired mandate, unapproved mission, cross-tenant mandate id, disabled identity, depth-0 delegate, and pending approval without a workflow grant.

## Tests executed and results

Command: `CGO_ENABLED=0 go test ./... -count=1 -timeout 5m`

CGO was disabled because the local macOS 27 SDK linker rejected `arm64e.x1` when linking OPA’s optional CGO path. The Decision API and policy engine do not require CGO.

| Package | Result |
| --- | --- |
| `github.com/iamredencio/agent-authority/cmd/decisiond` | **compiled** (no test files) |
| `github.com/iamredencio/agent-authority/internal/decision` | **PASS** |
| `github.com/iamredencio/agent-authority/internal/domain` | **PASS** |
| `github.com/iamredencio/agent-authority/internal/policy` | **PASS** |
| `github.com/iamredencio/agent-authority/internal/postgres` | **PASS** |

Phase 1 and Phase 2 regression tests remain green.

## PostgreSQL integration-test results

PostgreSQL was available via Docker (`postgres:16-alpine`) started by the existing test helper when `TEST_DATABASE_URL` was unset.

| Test | Result |
| --- | --- |
| `TestDecisionPersistenceRoundTrip` | PASS |
| `TestDecisionAPIPersistsDenyWithoutMandate` | PASS |
| `TestDecisionAPIUnknownOrganizationIsNotStored` | PASS |
| `TestIdentityBindingBySubject` | PASS |
| Phase 1–2 persistence suite | PASS |

## Lint / static analysis

| Check | Result |
| --- | --- |
| `gofmt -l .` | PASS (no unformatted files) |
| `go vet ./...` | PASS |
| `staticcheck ./...` (`honnef.co/go/tools` v0.8.1) | PASS (no findings) |

## OpenAPI

`api/openapi.yaml` documents `POST /v1/decisions` with act types `communicate` / `execute` / `delegate` / `spend` / `control`, results `allow` / `deny` / `pending_approval`, and identity as a claim. A missing or unknown `organization_id` is documented as HTTP `400` and is not a completed authorization decision. `TestOpenAPIDescribesDecisionContract` and `TestHTTPDecisionAPI` check the contract against the handler.

## Files changed

Updated:

- `docs/ROADMAP.md` — Phase 3 state `approved`; current implementation phase set to Phase 3; Phase 0–2 remain `locked`
- `README.md` — current status now records Phase 2 locked and Phase 3 approved
- `docs/ARCHITECTURE.md` — §9 points to D-021 for embedded OPA
- `docs/DECISIONS.md` — D-021 embedded OPA
- `go.mod` / `go.sum` — OPA v1.4.2 and OpenTelemetry
- `internal/domain/errors.go` — Phase 3 sentinel errors
- `internal/postgres/identity_bindings.go` — subject lookup for identity claims

Added:

- `internal/domain/decision.go` and `decision_test.go`
- `internal/domain/match.go`
- `internal/policy/`
- `internal/decision/`
- `internal/postgres/decisions.go` and `decision_test.go`
- `internal/postgres/migrations/000002_decisions.up.sql` / `.down.sql`
- `api/openapi.yaml`
- `cmd/decisiond/main.go`
- `policies/default.rego`
- `reports/PHASE-3-VERIFICATION-REPORT.md`

No `internal/adapters`, `internal/evidence`, Redis, SDK, Docker/Helm, or mandate signing packages were added.

## Database migrations

`000002_decisions` adds the evidence-ready `decisions` table. `evidence_record_id` is nullable and unused. `mandate_id` / `mission_id` / `actor_binding_id` are nullable so a deny can be stored when authentication has no matching mandate. There is no hash chain.

## Explicit Phase 3 non-goals left untouched

- MCP or A2A adapters
- Hash-chained evidence plane / WORM backends
- OIDC / Entra / Okta / SPIFFE provider integrations
- Human approval workflow (Phase 3 only returns `pending_approval`)
- Revocation or containment engine
- Payment execution / payment-rail adapters
- Registry federation / counterparty trust
- SDKs
- Redis
- Portable mandate wire format or cryptographic signing
- Phase 4 scaffolding (`internal/evidence`, `internal/adapters`)

## Residual risks or documentation gaps

- Unknown constraint keys still fail closed. A later decision may define a richer runtime constraint vocabulary.
- `pending_approval` is an outcome only. There is no grant/deny workflow (Phase 7).
- Mandate `state=revoked` is honored, but out-of-band revocation/containment is still Phase 8.
- Spend decisions do not decrement remaining budget (Phase 9).
- A missing or unknown `organization_id` is a `400` request error, not a stored deny. Completed communicate/execute/delegate/spend decisions against an existing tenant are always persisted.
- Mandate wire format and cryptographic signing profile remain unspecified (D-014).
- Integration tests start PostgreSQL with Docker when `TEST_DATABASE_URL` is unset. That is a test fixture, not product packaging.
- Local linking of OPA with CGO required `CGO_ENABLED=0` on this macOS 27 SDK. The process is intended to run without CGO.

## Phase 4 has not been started

**Phase 4 has NOT been started.** It remains `planned` and is not approved.

No hash-chained evidence log, MCP adapter, or WORM backend was implemented. This report does not approve Phase 4.

Phase 3 is **not** marked `locked`. Human acceptance of this report is required before that state change.

This close-out stops after Phase 3.
