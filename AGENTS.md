# AGENTS.md

This file is the engineering-agent contract for the `agent-authority` repository (codename **European AgentOS**).

If this file conflicts with a chat prompt, this file wins unless a human explicitly amends these documents in the same change set.

## Role

You implement only what is explicitly approved in the repository documentation.

You are not a product strategist, market-follow bot, or framework author. You do not invent adjacent products (agent runtimes, MCP gateways, IAM platforms, payment issuing, proprietary registries).

## Authoritative sources (in order)

When documents appear to disagree, resolve in this order:

1. `docs/SPECIFICATION.md` — product requirements (what must be true)
2. `docs/DECISIONS.md` — approved choices (why a specific realization was selected)
3. `docs/ROADMAP.md` — what may be built in the current phase
4. `docs/ARCHITECTURE.md` — how the approved specification is structured
5. `AGENTS.md` — how you work
6. `docs/WORKFLOW.md` — handoff and review process
7. `README.md` — orientation only
8. `docs/MARKET-WATCH.md` — informational only; never a build license

A GitHub Issue is a **work order**, not a higher source of product truth. It is valid only when it is consistent with the sources above and records explicit human approval. If an Issue conflicts with repository documentation, stop and report the conflict.

A market-watch finding is **not** a requirement. It becomes actionable only after it is recorded as an approved decision and/or an approved roadmap item.

## Before any implementation

Complete this checklist. If any item fails, stop and report.

1. Read this file in full.
2. Read `docs/SPECIFICATION.md`, `docs/ARCHITECTURE.md`, `docs/ROADMAP.md` and `docs/DECISIONS.md`.
3. Read `docs/WORKFLOW.md` when the task is handed off through GitHub.
4. Identify the **single currently approved implementation phase** in the roadmap.
5. Identify the single approved GitHub Issue/work order when one is supplied.
6. Confirm the Issue/work order records explicit human approval and is consistent with the approved phase.
7. Confirm the requested work is inside that phase’s scope and acceptance criteria.
8. Confirm the work is not an unapproved market-watch item.

If multiple candidate work orders exist, do not choose autonomously. Require an explicit Issue reference or human selection.

Phase 0 is documentation only. Application code is forbidden until Phase 1 is approved and explicitly requested.

## Phase discipline

For every implementation phase:

1. Read `AGENTS.md`.
2. Read the specification, architecture and roadmap.
3. Read the approved work-order Issue when the workflow uses one.
4. Implement **only** the currently approved phase/work order.
5. Add tests that cover the phase acceptance criteria.
6. Run linting and tests.
7. Produce `reports/PHASE-N-VERIFICATION-REPORT.md`.
8. Open/prepare a PR that links the work order and verification report when GitHub workflow is available.
9. **Stop.**
10. Do not proceed to the next phase without explicit human approval.

Do not “prepare”, “scaffold ahead”, or land types, APIs, adapters or schemas whose only purpose is a future phase. Forward-compatible seams that the current phase’s architecture already requires are allowed; speculative features are not.

### Verification report

Each `reports/PHASE-N-VERIFICATION-REPORT.md` must include:

- phase number and title
- specification sections implemented
- acceptance criteria, each marked pass/fail with evidence
- tests run and results
- lint results
- files changed
- explicit non-goals left untouched
- residual risks or documentation gaps
- statement that the next phase was **not** started

## What you must not do

- Do not write production application code during Phase 0.
- Do not start Phase *N+1* after finishing Phase *N*.
- Do not implement market-watch findings automatically.
- Do not position or build this repository as:
  - an agent framework or runtime
  - an MCP gateway
  - generic “agent security”
  - IAM for agents
  - an AI credit card or payment issuer
  - a proprietary agent registry as the end product
- Do not treat gateway authentication as authorization.
- Do not allow privilege amplification in delegation.
- Do not make the agent runtime or any gateway the sole source of truth for authority or evidence.
- Do not add Redis, extra languages, extra services or extra vendors unless an approved decision and the current phase require them.
- Do not add Python/TypeScript SDKs before the roadmap phase that approves them.
- Do not weaken fail-closed behaviour on unknown communication or execution paths.
- Do not commit secrets, live credentials, or production evidence.

## Product invariants (never regress)

These are always in force, including in early phases that only model them:

1. **Identity is an integration layer**, not a competing IdP.
2. **Authority is traceable** to an authority source.
3. **No autonomous action without an approved mission.**
4. **Delegation only attenuates.** A child MUST be no broader than its parent on every authority axis. Equality is allowed unless the specification requires a strict reduction. `delegation_depth` MUST always strictly decrease. Privilege amplification is forbidden.
5. **Communication and execution are independently authorized.** Unknown paths fail closed.
6. **Runtimes and gateways are untrusted enforcement environments.**
7. **Evidence is independent** of the runtime that performed the action.
8. **Revocation does not require runtime cooperation.**
9. **Financial authority is authorization, not issuing.** Payment rails are adapters.
10. **Mandates are portable** and must remain vendor-neutral.

## Stack constraints

Until a later approved decision supersedes them:

| Area | Rule |
| --- | --- |
| Backend language | Go |
| System of record | PostgreSQL |
| Policy | Initially compatible with OPA/Rego |
| Cache/broker | Redis only when justified by a decision |
| Observability | OpenTelemetry |
| Packaging | Docker; Kubernetes-ready |
| Evidence | Hash-chained append-only log |
| SDKs | Later; Python and TypeScript only when the roadmap says so |

Intended Go layout (create only when the approved phase needs it): `cmd/`, `internal/`, `api/`, `docs/`, `reports/`, `deploy/`.

## Git workflow

- **Never commit or push to `main`.** Create a feature branch for every change.
- Open a pull request into `main`. Do not merge unless the human explicitly asks.
- The repository auto-deletes head branches after merge. Do not keep long-lived feature branches.
- Branch names should be short and purpose-based (for example `docs/phase-0-foundation`, `phase-1-domain-model`).
- Do not force-push to `main`. Do not update git config.
- When work originates from an approved GitHub Issue, link that Issue from the PR.
- An Issue may carry implementation detail but may not override specification, decisions, roadmap, architecture, or this contract.

## Code and change rules (implementation phases)

- Prefer small, reviewable diffs that map to one phase.
- Exported APIs and mandate fields require specification coverage.
- Fail closed. Deny on missing mission, expired mandate, unknown destination, unknown action, broken delegation chain, or stale revocation state.
- Tests must include attenuation, expiry, fail-closed communication, and “authn ≠ authz” cases as soon as those modules exist.
- Do not skip hooks. Do not commit unless the human asked.
- After implementation, update only the docs that the change made wrong. Do not silently expand the specification to match extra code.

## If you are blocked

Stop. Record the gap in the verification report, GitHub Issue, PR, or chat. Propose a decision or roadmap amendment. Do not guess a product expansion.

## Phase 0 special rule

This task is documentation foundation only:

- create and maintain the files listed in the Phase 0 acceptance criteria
- do not create production application code
- do not start Phase 1
