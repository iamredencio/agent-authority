# Market watch

**Status:** Informational only  
**Not a specification. Not a roadmap. Not a build license.**

This file records landscape observations that may later inform product thinking. It has **no implementation authority**.

Per [DECISIONS.md](DECISIONS.md) **D-017** and [AGENTS.md](../AGENTS.md):

- Market-watch findings MUST NOT be implemented automatically.
- A finding becomes actionable only after it is recorded as an **accepted decision** in `docs/DECISIONS.md` and/or as an **approved roadmap item** in `docs/ROADMAP.md`.
- Engineering agents that add code, schemas or adapters “because the market moved” are out of contract.

---

## Promotion process

1. Someone adds or updates a **Finding** below (observation + sources + date).
2. A human decides whether it matters to Agent Authority.
3. If yes, they write a decision (and, if needed, a roadmap amendment).
4. Only then may a later implementation phase consume it.

Until step 3 happens, the correct engineering response is: **do nothing**.

---

## Watch domains

| Domain | Why we watch | Typical non-response |
| --- | --- | --- |
| Agent-to-tool protocols (MCP and successors) | Adapter targets; untrusted enforcement | Do not become a gateway |
| Agent-to-agent protocols (A2A and peers) | Communication-authority targets | Do not become a runtime |
| Enterprise identity (OIDC, Entra, Okta, SPIFFE) | Identity bindings | Do not become an IdP |
| Policy engines | PDP implementation options | Do not let policy amplify mandates |
| Payment rails (cards, SEPA, Stripe, x402) | Spend adapters | Do not issue instruments |
| Agent directories / KYA | Federation inputs | Do not ship a proprietary registry |
| “Agent IAM” and MCP auth products | Positioning threats and complements | Do not collapse into generic IAM |
| EU/regulatory (AI Act, eIDAS, audit) | Deployment constraints | Do not claim certification |

---

## Findings

Findings are observations. `Implication` is analysis, not work.

### MW-001 — MCP remains the dominant agent-to-tool protocol

**Date:** 2026-09-06  
**Domain:** Agent-to-tool  
**Status:** observed

**Observation.** Model Context Protocol (MCP) is the primary open standard for connecting agents to tools, APIs and resources. Enterprise deployments are adding authorization extensions around MCP rather than replacing it.

**Sources.** [modelcontextprotocol.io](https://modelcontextprotocol.io); MCP authorization extension docs.

**Implication.** Phase 5 (MCP adapter) remains the correct first protocol adapter. This does **not** authorize building an MCP gateway product.

**Promoted?** No.

---

### MW-002 — MCP Enterprise-Managed Authorization (EMA) is identity/connection, not mission authority

**Date:** 2026-09-06  
**Domain:** Agent IAM / MCP auth  
**Status:** observed

**Observation.** The MCP extension `io.modelcontextprotocol/enterprise-managed-authorization` was declared stable on 2026-06-18. It lets an enterprise IdP (initially Okta; ID-JAG / RFC 8693 + RFC 7523) decide which clients may reach which MCP servers, reducing per-server consent. Commentary on the spec notes that the IdP does not see MCP traffic and does not make a per-call, context-aware decision.

**Sources.**

- [MCP EMA extension](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/main/docs/extensions/auth/enterprise-managed-authorization.mdx)
- [Analysis: what EMA leaves to you](https://ehosseini.info/articles/mcp-enterprise-managed-authorization-ema/)

**Implication.** EMA reinforces — it does not replace — Agent Authority. Connection/authn to an approved server is still not execution authority, mission binding, attenuation, evidence or out-of-band revocation. If a later decision wants EMA compatibility, that is an identity-adapter concern (near Phase 6), not a reason to shrink the PDP.

**Promoted?** No.

---

### MW-003 — A2A 1.0 is an interoperability protocol, not an authority plane

**Date:** 2026-09-06  
**Domain:** Agent-to-agent  
**Status:** observed

**Observation.** Agent2Agent (A2A) is an open protocol (Linux Foundation; originated at Google) for discovery and task exchange between opaque agents. Specification 1.0.0 is published. Agent Cards at `/.well-known/agent-card.json` are the discovery document. MCP and A2A are positioned as complementary (tools vs agents). Other protocols (IBM ACP, community ANP) exist; A2A is the current enterprise-facing default to watch. Microsoft Work IQ documents A2A 1.0 with Entra-delegated auth.

**Sources.**

- [A2A protocol](https://a2a-protocol.org/latest/)
- [A2A 1.0 specification](https://a2a-protocol.org/v1.0.0/specification/)
- [Work IQ A2A overview](https://learn.microsoft.com/en-us/microsoft-365/copilot/extensibility/work-iq/a2a/overview)

**Implication.** A2A destinations belong under **communication authority**. An A2A adapter is explicitly unscheduled (roadmap). Do not implement A2A in Phases 1–5. Do not treat Agent Cards as mandates.

**Promoted?** No.

---

### MW-004 — x402 is a payment rail, not financial authority

**Date:** 2026-09-06  
**Domain:** Payment rails  
**Status:** observed

**Observation.** x402 is an open HTTP 402 “Payment Required” protocol for machine payments (stablecoins and related rails; Coinbase-originated, now with a foundation). It prices, requests and settles a payment once a client already has spend capability. Public explainers distinguish the **wallet/authority** layer (who may spend) from the **x402** layer (how a specific payment is communicated and settled).

**Sources.**

- [x402](https://x402.org/)
- [x402 whitepaper (2026-06)](https://x402.org/wp-content/uploads/sites/10/2026/06/x402-whitepaper.pdf)
- [MetaMask: what is x402](https://metamask.io/news/what-is-x402)

**Implication.** Consistent with D-009 and Phase 9: x402 is a candidate **adapter**, never the core. Seeing x402 adoption is not a reason to add crypto wallets, facilitators or issuing to this repository.

**Promoted?** No.

---

### MW-005 — Enterprise identity vendors remain the identity source, not the product

**Date:** 2026-09-06  
**Domain:** Identity  
**Status:** observed

**Observation.** Entra ID, Okta (including Cross App Access / ID-JAG), SPIFFE/SPIRE and workload identity continue to be how enterprises name humans and workloads. CrowdStrike and similar vendors appear as additional telemetry/identity-adjacent sources in some environments.

**Implication.** Phase 6 (OIDC/Entra-compatible adapter) is the approved first identity adapter. Okta- and SPIFFE-specific packages stay unscheduled. EMA (MW-002) may eventually inform how bindings are proven; it does not change D-002.

**Promoted?** No.

---

### MW-006 — Adjacent “agent IAM” and gateway products will keep appearing

**Date:** 2026-09-06  
**Domain:** Positioning  
**Status:** standing watch

**Observation.** Vendors will ship MCP gateways, agent firewalls, tool allow-lists, and “IAM for agents”. Many will authenticate sessions or constrain servers. Few will bind acts to a traceable authority source, an approved mission, an attenuating portable mandate, an independent evidence plane and out-of-band containment together.

**Implication.** Treat overlap as positioning, not as a feature race. If a named vendor later matters for partnership or federation, add a dated finding and promote it through D-017. Do not clone gateway feature lists.

**Promoted?** No.

---

## Standing questions (do not implement answers)

These are prompts for later human decisions, not backlog items:

1. When Phase 5 starts, should the MCP adapter speak EMA as a *claim source* only?
2. When (if ever) should an A2A adapter be scheduled relative to Phase 10 federation?
3. Which first payment adapter, if any, is justified in Phase 9 — fake port, Stripe, SEPA, or x402?
4. Is a signed mandate format JWS, or something else, at the Phase 5 gate (D-014)?
5. Do EU deployment customers need a residency/WORM profile before Phase 4 storage stays on PostgreSQL only?

---

## Change log

| Date | Change |
| --- | --- |
| 2026-09-06 | Initial informational watch file; MW-001–MW-006 recorded; none promoted. |
