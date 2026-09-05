# Decisions

**Status:** Architecture and product decision log (ADR-style)  
**Product:** Agent Authority

A decision here is the only thing that can authorize a departure from a previous choice. Market-watch entries are not decisions.

Format:

- **ID** — stable
- **Title**
- **Status** — `accepted` | `superseded` | `proposed`
- **Date**
- **Context**
- **Decision**
- **Consequences**

---

## D-001 — Independent authority and evidence product

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Autonomous agents are appearing behind MCP, A2A, API gateways and payment rails. Each of those layers can authenticate and some can enforce locally. None of them independently answers *why this agent may act now*, nor do they provide evidence that survives a compromised runtime.

**Decision.** Build **independent authority and evidence infrastructure for autonomous AI**. Do not build an agent framework, MCP gateway, generic agent-security suite, IAM-for-agents product, AI credit card, or proprietary registry as the end product.

**Consequences.** Integrations are adapters. The moat is mandate semantics, attenuation, independent decisions and evidence. Feature requests that pull the product into a runtime or gateway must be rejected or recorded as a new decision.

---

## D-002 — Identity is an integration layer

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Entra ID, Okta, SPIFFE, CrowdStrike, workload identity and custom IdPs already issue identities.

**Decision.** Agent Authority stores **identity bindings** to external subjects. It does not become an identity provider. Authentication is never sufficient for authorization.

**Consequences.** Phase 6 adds OIDC/Entra-compatible verification. Other IdP adapters need their own phase or decision. No proprietary global agent ID is required to issue a mandate.

---

## D-003 — Mission-bound authority (“why now”)

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Capability tokens and RBAC can say *what* is allowed. They rarely say *why this is allowed at this moment*.

**Decision.** Every autonomous action must be associated with an **approved mission** inside its validity window. Mandates reference that mission. Ending the mission disables derived mandates.

**Consequences.** Domain model and decision path always load mission state. There is no “standing unconstrained agent admin role” that bypasses missions.

---

## D-004 — Attenuation-only delegation

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Multi-agent systems spawn child agents. If children can widen scope, depth, budget or destinations, the original grant is meaningless.

**Decision.** A child mandate MUST be **no broader** than its parent on every authority axis in specification §14.3. **Equality is allowed** on an axis unless the specification explicitly requires a strict reduction. **`delegation_depth` MUST always strictly decrease.** Privilege amplification is a hard error. Delegation does **not** require every axis to become strictly narrower.

**Consequences.** Phase 2 must test amplification (widening) attempts and must accept equal-on-axis children where §14.3 allows equality. Policy (Phase 3) may further deny but never widen.

---

## D-005 — Runtimes and gateways are untrusted

**Status:** accepted  
**Date:** 2026-09-06

**Context.** MCP servers, A2A endpoints, API gateways and agent runtimes will be compromised, misconfigured or simply buggy.

**Decision.** Treat them as **untrusted enforcement environments**. Each communicate/execute/delegate/spend act requires an independent Decision API result. Fail closed on unknown paths. Gateway authentication is not authorization.

**Consequences.** Adapters ask; they do not decide. Cached allows are bounded and revocation-sensitive (specification S-EA-5).

---

## D-006 — Own enforcement and evidence; adopt vendors elsewhere

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Re-implementing identity, payments, registries or protocol gateways would create a framework/IAM/fintech product.

**Decision.** Embrace existing standards and vendors. Own the **authority enforcement** and **evidence** layer. Initial policy compatibility is **OPA/Rego**. Embedded vs sidecar OPA is deferred to Phase 3 (record a follow-up decision then).

**Consequences.** Core must test without vendor SDKs. Policy must not amplify mandates.

---

## D-007 — Independent hash-chained evidence plane

**Status:** accepted  
**Date:** 2026-09-06

**Context.** If the runtime is the only log, a compromised agent can rewrite history.

**Decision.** Record mission, principal, agent, chain, decision, approvals, calls, results, revocation and containment in an **append-only hash-chained** log owned by Agent Authority. Support later signing and WORM backends without changing record semantics.

**Consequences.** Phase 3 stores decisions in a form Phase 4 can chain. From Phase 4, required evidence append failures fail closed.

---

## D-008 — Out-of-band revocation

**Status:** accepted  
**Date:** 2026-09-06

**Context.** A hostile or offline runtime will not honor “please stop”.

**Decision.** Revocation and containment take effect in the authority plane **without runtime cooperation**. Adapter fan-out is best-effort and must not delay local deny.

**Consequences.** Phase 8 implements this. Earlier phases still model `revoked` as a terminal mandate state so the decision path can honor it once present.

---

## D-009 — Financial authority is authorization, not issuing

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Machine payments (cards, SEPA, x402, Stripe) will be used by agents. Issuing instruments would make this a fintech product and a PCI/scope problem.

**Decision.** The core decides **whether and how much** an agent may spend. Payment rails are adapters. No PAN vault, no card issuing, no settlement engine.

**Consequences.** Phase 9 adds budget + `spend` decisions + a port. Specific rails need explicit later decisions.

---

## D-010 — No proprietary registry as the product

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Cloud and community agent directories will exist. Owning “the” registry is a different business and creates lock-in.

**Decision.** Do not build a proprietary agent registry as the end product. Federate and remain portable across external registries (Phase 10).

**Consequences.** A local cache of attestations is allowed. Exclusive registration in Agent Authority is forbidden as a functional requirement.

---

## D-011 — Counterparty trust is a later extension

**Status:** accepted  
**Date:** 2026-09-06

**Context.** KYA / Know Your Agent and counterparty credentials matter, but they are not required to deliver first-party authority.

**Decision.** Leave an explicit extension point. Do not pretend counterparties are verified until Phase 10 (or a successor decision) implements it. When enabled, unmet required checks fail closed.

**Consequences.** Phases 1–9 may omit counterparty evaluation. They must not store a fake “verified” flag.

---

## D-012 — Implementation stack

**Status:** accepted  
**Date:** 2026-09-06

**Context.** The control plane must be boring, auditable and operable in European enterprise environments.

**Decision.**

| Choice | Value |
| --- | --- |
| Backend | Go |
| Store | PostgreSQL |
| Policy | OPA/Rego-compatible |
| Redis | Only when a later decision justifies it |
| Telemetry | OpenTelemetry |
| Packaging | Docker, Kubernetes-ready |
| SDKs | Python and TypeScript, only when scheduled |
| Decision API | HTTP/JSON in Phase 3 |
| Time | UTC, RFC 3339 |
| IDs | UUID |

**Consequences.** No extra language, broker or database in Phases 1–4. SDKs stay unscheduled (see roadmap).

---

## D-013 — Multi-tenant organization boundary

**Status:** accepted  
**Date:** 2026-09-06

**Context.** The thesis is enterprise infrastructure with multiple IdPs and principals. Isolation must be explicit.

**Decision.** Introduce `organization` as the tenancy and evidence-chain boundary. All aggregates are organization-scoped. Cross-tenant reads fail closed.

**Consequences.** This is an architectural assumption not named in the original thesis list; it is required to implement safely. Single-tenant deploy is just one organization.

---

## D-014 — Mandate serialization and signing timing

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Portable mandates need a stable wire format and a cryptographic signing profile. Choosing those now would either block the Phase 1 logical model or lock an unreviewed scheme.

**Decision.** Phase 1 persists the **logical** mandate only. Internal persistence of that model is not a portable wire format. This decision does **not** select a mandate wire format and does **not** select a cryptographic signing profile (including JWS, COSE, VC, or any other scheme).

**Before Phase 5 implements an external MCP adapter that relies on portable mandates**, a **separate accepted decision** MUST define both:

1. the mandate wire format
2. the cryptographic signing profile

Until that separate decision exists and is `accepted`, Phase 5 MUST NOT implement or depend on a portable signed mandate document. An MCP adapter may still ask the Decision API using internal mandate identifiers.

**Consequences.** Internal Phase 1–4 tests may use unsigned persisted objects. Phase 5 cannot treat mandates as portable interchange documents by inventing a format or signature suite in code. Architecture and roadmap gates refer here; they do not fill in the missing decision.

---

## D-015 — Evidence storage starts in PostgreSQL

**Status:** accepted  
**Date:** 2026-09-06

**Context.** WORM object lock is desirable but would block the evidence plane on a vendor and ops model.

**Decision.** Phase 4 implements the hash chain in PostgreSQL. WORM/object-lock is a later storage adapter with identical record semantics. One chain per organization. Algorithm: SHA-256.

**Consequences.** Immutability is first enforced by application append-only rules and chain verification, then by storage controls.

---

## D-016 — Phased, gated delivery

**Status:** accepted  
**Date:** 2026-09-06

**Context.** A twelve-concept platform can collapse into an un-auditable monolith if built “all at once”.

**Decision.** Deliver Phases 0–10 as in `docs/ROADMAP.md`. Each implementation phase ends with tests, lint and `reports/PHASE-N-VERIFICATION-REPORT.md`, then **stops**. The next phase requires explicit human approval. Phase 0 is documentation only.

**Consequences.** Engineering agents must not scaffold future phases. Phase 3 is evidence-ready; Phase 4 is the evidence plane. MCP is Phase 5, not the product core.

---

## D-017 — Market-watch is informational only

**Status:** accepted  
**Date:** 2026-09-06

**Context.** The agent, MCP, payment and identity landscape will move quickly. Chasing it from a watch list would destroy the product thesis.

**Decision.** `docs/MARKET-WATCH.md` is **informational**. Findings MUST NOT be implemented automatically. A finding becomes actionable only after it is written as an **accepted decision** and/or an **approved roadmap item**.

**Consequences.** “Vendor X launched Y” is never sufficient reason to add code. Agents that implement from market-watch alone are out of contract.

---

## D-018 — Codename vs product name

**Status:** accepted  
**Date:** 2026-09-06

**Context.** The repository is `agent-authority`. The working codename is European AgentOS.

**Decision.** External and specification name: **Agent Authority**. Codename: **European AgentOS**. Design should remain compatible with regulated European deployments (auditability, vendor neutrality, residency-as-deployment). No certification is claimed.

**Consequences.** Do not rename packages to AgentOS. Do not add AI Act/eIDAS feature work without a new decision.

---

## D-019 — Phase 0 creates no application code

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Specification work is easy to mix with “just a skeleton module”.

**Decision.** Phase 0 adds only the documentation files listed in the roadmap. No Go module, no schema, no CI application pipeline required to complete Phase 0.

**Consequences.** The first `go.mod` belongs to Phase 1 after explicit approval.

---

## D-020 — Feature branches, pull requests, auto-delete on merge

**Status:** accepted  
**Date:** 2026-09-06

**Context.** Direct commits to `main` skip review and leave stale branches after merge.

**Decision.** Every change lands on a feature branch and a pull request to `main`. Do not push implementation or documentation commits to `main`. Enable GitHub **Automatically delete head branches** so merged branches are removed.

**Consequences.** Engineering agents must create a branch before committing or pushing. Local `main` should track `origin/main`. Long-lived feature branches are not used.
