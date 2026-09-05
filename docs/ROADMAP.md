# Roadmap

**Status:** Phase plan and acceptance criteria  
**Product:** Agent Authority  
**Normative parent:** [SPECIFICATION.md](SPECIFICATION.md)

This document defines **what may be built**, in which order, and how a phase is accepted. It does not authorize skipping ahead.

Market-watch items are not phases. They enter this file only after an approved decision.

---

## How to read this file

| Column | Meaning |
| --- | --- |
| **State** | `approved` — may be implemented when a human explicitly requests it; `locked` — done; `planned` — not yet approved for implementation |
| **Gate** | A human must explicitly approve starting the next phase after the verification report for the current phase |

**Current approved implementation phase:** Phase 0 only.

Phase 1+ remain `planned` until a human marks them `approved` **and** explicitly requests implementation.

---

## Phase rules (every implementation phase)

1. Read `AGENTS.md`.
2. Read specification, architecture and this roadmap.
3. Implement only this phase.
4. Add tests.
5. Run linting and tests.
6. Produce `reports/PHASE-N-VERIFICATION-REPORT.md`.
7. Stop.
8. Do not start the next phase without explicit approval.

---

## Phase 0 — Repository and specification foundation

**State:** `approved` (this change set)  
**Goal:** Establish the product contract. No application functionality.

### In scope

- `README.md`
- `AGENTS.md`
- `docs/SPECIFICATION.md`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`
- `docs/DECISIONS.md`
- `docs/MARKET-WATCH.md`

### Out of scope

- Go modules, DDL, APIs, adapters, SDKs, deploy manifests
- Any Phase 1 domain implementation

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P0-1 | All seven files exist and are internally consistent. |
| P0-2 | Specification states positioning, twelve core concepts, mandate fields, and fail-closed rules. |
| P0-3 | Architecture defines planes, logical data model, trust boundaries and intended layout without shipping code. |
| P0-4 | Roadmap lists Phases 0–10 with acceptance criteria. |
| P0-5 | Decisions record initial ADRs, including “market-watch is not a build license”. |
| P0-6 | Market-watch is labeled informational only. |
| P0-7 | No production application code is added. |
| P0-8 | Engineering contract in `AGENTS.md` requires stop-after-phase and verification reports. |

---

## Phase 1 — Core authority domain model

**State:** `planned`  
**Goal:** Persist the core aggregates and invariants in Go + PostgreSQL. No public decision API.

### In scope

- Go module and `internal/domain`
- PostgreSQL schema for Organization, IdentityBinding, AuthoritySource, Mission, Mandate
- Constructors and invariant checks that can be unit-tested without adapters
- Migrations
- Mandate field coverage from specification §14 (logical model)

### Out of scope

- Delegation engine beyond storing `parent_mandate_id` and `delegation_depth`
- Policy/Decision API
- Evidence hash chain
- MCP, OIDC, payments
- Redis

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P1-1 | Aggregates in §6 of architecture can be created, read and persist-round-tripped. |
| P1-2 | Identity is stored only as a binding to an external reference. |
| P1-3 | A mandate cannot be created without mission, principal, agent and authority source. |
| P1-4 | Tests cover missing required fields and tenancy scoping. |
| P1-5 | Lint and tests pass. `reports/PHASE-1-VERIFICATION-REPORT.md` exists. |
| P1-6 | No Decision API, adapter or evidence-chain implementation. |

---

## Phase 2 — Mission and delegation engine

**State:** `planned`  
**Goal:** Enforce mission windows and attenuation-only delegation.

### In scope

- Mission state machine (`draft` → `pending_approval` → `approved` / …)
- Originating mandate issuance against an approved mission
- Child mandate issuance with full attenuation checks (specification §14.3)
- Delegation chain reconstruction
- Rejection of privilege amplification

### Out of scope

- HTTP Decision API
- Human approval workflow (record `pending_approval` only)
- Evidence chain
- Adapters

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P2-1 | Actions (as domain operations) refuse a non-approved or out-of-window mission. |
| P2-2 | Child issuance fails if any attenuation axis is widened. |
| P2-3 | `delegation_depth` 0 cannot issue children; child depth is strictly smaller. |
| P2-4 | Child expiry/not_before cannot exceed parent windows. |
| P2-5 | Ancestor walk fails closed on a missing link. |
| P2-6 | Lint, tests, `reports/PHASE-2-VERIFICATION-REPORT.md`. No Phase 3 API. |

---

## Phase 3 — Policy decision API

**State:** `planned`  
**Goal:** Independent authorization for each requested act.

### In scope

- HTTP/JSON Decision API
- OPA/Rego-compatible policy evaluation
- Results: `allow`, `deny`, `pending_approval`
- Authn treated as a claim, not as authorization
- OpenAPI description under `api/`
- OpenTelemetry instrumentation of the PDP path
- Evidence-ready decision persistence (hash chain still Phase 4)

### Out of scope

- MCP adapter
- Hash-chained evidence plane
- Identity provider integration beyond test fixtures
- Redis unless a new decision justifies it

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P3-1 | Each communicate/execute/delegate/spend request produces a stored decision. |
| P3-2 | Valid authentication without a matching mandate is `deny`. |
| P3-3 | Unknown destination or action is `deny` (fail closed). |
| P3-4 | Policy can deny a mandate-permitted act; policy cannot allow a mandate-forbidden act. |
| P3-5 | OpenTelemetry spans exist for decision evaluation. |
| P3-6 | Lint, tests, `reports/PHASE-3-VERIFICATION-REPORT.md`. No MCP adapter. |

---

## Phase 4 — Independent evidence plane

**State:** `planned`  
**Goal:** Hash-chained append-only evidence log, independent of runtimes.

### In scope

- `EvidenceRecord` persistence and SHA-256 chain per organization
- Record types in architecture §7
- Decision and mandate/mission lifecycle events append to the log
- Tamper-detection test (broken chain)
- Signature field present (signing backend may be stubbed if D-014 still open)
- Design seam for later WORM storage

### Out of scope

- MCP adapter
- External WORM vendor integration
- Full key-management service

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P4-1 | Required event types can be appended and replayed in order. |
| P4-2 | `record_sha256` covers canonical payload + `prev_sha256` + metadata. |
| P4-3 | A mutated historical record fails chain verification. |
| P4-4 | Runtime-only claims cannot be the sole record of authorization. |
| P4-5 | Unavailable evidence plane fails closed when the mandate requires evidence. |
| P4-6 | Lint, tests, `reports/PHASE-4-VERIFICATION-REPORT.md`. No Phase 5 adapter. |

---

## Phase 5 — MCP adapter

**State:** `planned`  
**Goal:** Treat MCP as an untrusted enforcement environment that must ask the Decision API.

### In scope

- MCP adapter package
- Signed/portable mandate representation if not already locked
- Map MCP tool calls to communication + execution decisions
- Fail closed on unknown servers/tools
- Evidence of MCP calls via the evidence plane

### Out of scope

- Shipping a general-purpose MCP gateway product
- A2A adapter
- Identity provider productization

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P5-1 | An MCP tool call is authorized only after Decision API `allow`. |
| P5-2 | Unknown MCP server or tool is denied. |
| P5-3 | Adapter cannot mint or widen a mandate. |
| P5-4 | MCP call and decision are evidenced. |
| P5-5 | Lint, tests, `reports/PHASE-5-VERIFICATION-REPORT.md`. |

---

## Phase 6 — OIDC / Entra-compatible identity adapter

**State:** `planned`  
**Goal:** Verify inbound identity from OAuth 2.x / OIDC / Entra-compatible issuers.

### In scope

- Inbound token/JWT verification adapter
- Mapping verified subjects onto `IdentityBinding`
- Entra-compatible claim mapping
- Still: authn ≠ authz

### Out of scope

- Becoming an IdP
- Okta/SPIFFE-specific adapters (may share interfaces; dedicated packages need a later phase or an explicit decision)
- Human approval UI

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P6-1 | A valid token binds to an existing IdentityBinding or a controlled just-in-time bind if explicitly configured. |
| P6-2 | An invalid, expired or wrong-audience token is rejected. |
| P6-3 | A valid token without a mandate still yields `deny` on the Decision API. |
| P6-4 | Lint, tests, `reports/PHASE-6-VERIFICATION-REPORT.md`. |

---

## Phase 7 — Human approval workflow

**State:** `planned`  
**Goal:** Satisfy `approval_requirements` with recorded human (or designated steward) approvals.

### In scope

- Approval API and state machine
- Mission and decision `pending_approval` resolution
- Evidence for grant/deny
- Notifications via a generic outbound port (email/webhook adapter optional)

### Out of scope

- Full end-user GUI unless a decision adds it
- Changing fail-closed defaults

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P7-1 | A mandate requiring approval cannot yield `allow` until a matching grant exists. |
| P7-2 | Denied approval yields `deny` and is evidenced. |
| P7-3 | Approver is an identity binding, not a free-text name only. |
| P7-4 | Lint, tests, `reports/PHASE-7-VERIFICATION-REPORT.md`. |

---

## Phase 8 — Revocation and containment

**State:** `planned`  
**Goal:** Out-of-band control that does not depend on runtime cooperation.

### In scope

- Revoke mandate, descendant mandates, mission
- Revoke derived adapter credentials and payment authority (ports)
- Containment actions: children, spend, network access (as adapter fan-out)
- Decisions deny immediately after revocation is recorded
- Evidence of control actions

### Out of scope

- Implementing every vendor fan-out
- Waiting on runtime acknowledgement to consider revocation effective

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P8-1 | Revoking a mandate denies subsequent decisions without calling the runtime. |
| P8-2 | Descendants and derived spend/comms authority are invalidated. |
| P8-3 | Adapter fan-out failure does not leave the mandate active in the authority plane. |
| P8-4 | Control events are evidenced. |
| P8-5 | Lint, tests, `reports/PHASE-8-VERIFICATION-REPORT.md`. |

---

## Phase 9 — Financial authority abstraction

**State:** `planned`  
**Goal:** Authorize spend; keep payment rails as adapters.

### In scope

- Budget on mandates
- `spend` act type on the Decision API
- Remaining-budget updates under concurrency control
- Payment adapter port (one reference adapter **or** a fake adapter if no vendor is approved)
- Evidence of spend decisions and observed results

### Out of scope

- Card issuing, PAN storage, becoming a money transmitter
- Implementing all of Stripe, Visa, Mastercard, SEPA, x402 (those are optional adapters behind the port)

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P9-1 | Spend without budget or with exhausted budget is `deny`. |
| P9-2 | Core package has no payment-rail issuer logic. |
| P9-3 | Adapter cannot spend without a prior `allow`. |
| P9-4 | Concurrent spend cannot exceed remaining budget. |
| P9-5 | Lint, tests, `reports/PHASE-9-VERIFICATION-REPORT.md`. |

---

## Phase 10 — Federation and external attestations

**State:** `planned`  
**Goal:** Portable trust across external registries; counterparty/KYA extension becomes real if still approved.

### In scope

- Registry adapter port
- Ingest and cache external attestations as claims
- Mandate verification that does not require a proprietary registry
- Optional counterparty evaluation hook (fail closed when required and unmet)
- Evidence of attestation use

### Out of scope

- A proprietary Agent Authority global directory as the product
- Automatically trusting any market registry

### Acceptance criteria

| ID | Criterion |
| --- | --- |
| P10-1 | A mandate remains verifiable with zero, one or many external registry entries. |
| P10-2 | Registry outage does not create new authority (fail closed only if a mandate *requires* a live attestation). |
| P10-3 | No exclusive proprietary registry is required to issue or decide. |
| P10-4 | Counterparty checks, if enabled, fail closed when unmet and never silently skip. |
| P10-5 | Lint, tests, `reports/PHASE-10-VERIFICATION-REPORT.md`. |

---

## Explicitly unscheduled

These need a decision before they become phases:

- Python and TypeScript SDKs
- Dedicated Okta, SPIFFE, CrowdStrike adapters
- A2A adapter
- Production WORM/object-lock backend
- Human GUI
- Redis
- Multi-region / active-active
- Certification programs (AI Act, eIDAS, PCI)

---

## Change control

- Adding, splitting or reordering phases requires a decision entry.
- Widening a phase after implementation has started requires a human-approved amendment in this file **before** code lands.
- Completing a phase changes its state to `locked` only after the verification report is accepted by a human.
