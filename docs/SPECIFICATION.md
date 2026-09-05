# Specification

**Status:** Authoritative  
**Product:** Agent Authority  
**Codename:** European AgentOS  
**Phase:** 0 (foundation)

This document is the authoritative product specification. Implementation, architecture and tests must conform to it. Roadmap phases may defer *when* a requirement is built; they may not contradict it.

---

## 1. Product thesis

Agent Authority is independent, vendor-neutral **authority, delegation, trust and evidence** infrastructure for autonomous AI agents.

It enables an organization to:

- bind an agent to a **principal**, a **mission** and an **authority source**
- issue a **portable agent mandate** that states what may be done, why, with what limits, and until when
- **delegate** that mandate to child agents only by attenuation
- **authorize** each communication and each execution independently of the agent runtime and of any gateway
- **record evidence** that can be verified without trusting the runtime
- **revoke and contain** authority out of band
- **decide spend authority** without becoming a payment issuer
- eventually evaluate **counterparty trust** and **federate** with external registries

### 1.1 Positioning

The product **is**:

> Independent authority and evidence infrastructure for autonomous AI.

The product **is not**:

| Anti-position | Why it is rejected |
| --- | --- |
| Agent framework / runtime | We do not execute agent loops, tools or models. |
| MCP gateway | MCP is an adapter target. Gateways are untrusted enforcement. |
| Generic agent security | Security tooling without mission-bound, source-traced authority is out of scope. |
| IAM for agents | Identity is an integration layer. We consume identities; we do not replace IdPs. |
| AI credit card | We authorize spend. We do not issue cards, hold balances as a bank, or own payment rails. |
| Proprietary agent registry | Registries are integration targets. Portability across registries is the goal. |

### 1.2 Architecture principle

Embrace existing standards and vendors; own the **authority enforcement** and **evidence** layer.

---

## 2. Terminology

| Term | Meaning |
| --- | --- |
| **Principal** | The party that originates or holds accountable authority (human, organization role, or accountable system). |
| **Agent** | An autonomous software actor that acts under a mandate. |
| **Identity reference** | A pointer to an identity mastered in an external provider (Entra ID, Okta, SPIFFE, workload identity, custom IdP). |
| **Authority source** | The original, traceable origin of a grant (see §4). |
| **Mission** | An approved statement of purpose that explains why authority exists *now*. |
| **Mandate** | The portable, machine-readable authority object (see §14). |
| **Delegation** | Issuance of a child mandate derived from a parent mandate. |
| **Attenuation** | A child mandate that is no broader than its parent on every authority axis. Equality is allowed unless an axis explicitly requires a strict reduction. |
| **Communication authority** | Permission to open a channel to a named class of destination. |
| **Execution authority** | Permission to perform a named action (tool, API, payment, side effect). |
| **Decision** | An independent allow/deny (or pending-approval) result for one requested act. |
| **Evidence record** | An externally stored, hash-chained attestation of a fact in the authority lifecycle. |
| **Revocation** | Immediate invalidation of a mandate or derived credential, independent of the runtime. |
| **Containment** | A control action that restricts an agent’s remaining ability to act (network, children, spend, mission). |
| **Financial authority** | Permission and limits to instruct a payment adapter to spend. |
| **Counterparty** | An external agent, service or organization on the other side of a communication or transaction. |
| **Fail closed** | If authority cannot be positively established, the decision is deny. |
| **Untrusted enforcement environment** | Any runtime, gateway, MCP server, A2A endpoint or SDK that might be compromised; it may *ask* and *enforce locally*, but is never the system of record. |

---

## 3. Identity

Identity **may** come from upstream providers such as Entra ID, Okta, CrowdStrike, SPIFFE/SPIRE, cloud workload identity, or custom IdPs.

Identity **is an integration layer, not the primary moat.**

### Requirements

- **S-ID-1.** The system SHALL represent principals and agents as local bindings to external **identity references**, not as a competing identity provider.
- **S-ID-2.** A binding SHALL record provider, subject identifier, and optional attestation metadata sufficient to re-verify the reference.
- **S-ID-3.** Authentication of a caller SHALL NOT, by itself, authorize any action.
- **S-ID-4.** The system SHALL support multiple identity providers per organization.
- **S-ID-5.** The system MUST NOT require a proprietary global agent identifier as a condition of issuing a mandate.

---

## 4. Authority source

Authority **must** be traceable to an original source.

Examples (non-exhaustive): human approval, role, policy, contract, procurement mandate, board mandate, credential, machine-readable agreement.

### Requirements

- **S-AS-1.** Every mandate SHALL reference exactly one primary authority source, and MAY reference supporting sources.
- **S-AS-2.** An authority source SHALL have a type, an identifier, a recorded issuer or steward, and a retrieval or evidence pointer.
- **S-AS-3.** A decision that cannot name the authority source of the governing mandate SHALL fail closed.
- **S-AS-4.** Replacing or widening an authority source SHALL require a new mandate, not an in-place amplification.

---

## 5. Mission

Every autonomous action must be associated with an **approved mission**.

The mission explains **why** the authority exists. Core principle:

> Do not only prove what an agent may do; prove why it may do it right now.

### Requirements

- **S-MS-1.** A mission SHALL include purpose, principal, intended outcome, validity window, and approval state.
- **S-MS-2.** The system SHALL NOT authorize an autonomous action unless it is associated with a mission in an approved state and within its validity window.
- **S-MS-3.** A mandate SHALL reference the mission that justifies it.
- **S-MS-4.** Ending, suspending or expiring a mission SHALL make derived mandates non-authorizing.
- **S-MS-5.** “Right now” SHALL be evaluated with the mandate validity window, mission validity window, and any time constraints; all must hold at decision time.

---

## 6. Delegation

Agents may delegate authority to child agents.

Delegation **must always attenuate** authority: the child MUST be no broader than its parent on every authority axis. Equality is allowed on an axis unless this specification requires a strict reduction. **`delegation_depth` MUST always strictly decrease.** **Privilege amplification is forbidden.**

Delegation depth, expiry, scope and constraints must be explicit.

### Requirements

- **S-DL-1.** A child mandate SHALL reference its parent mandate.
- **S-DL-2.** Delegation SHALL succeed only if the child is an attenuation of the parent on every authority axis in §14.3.
- **S-DL-3.** Remaining delegation depth on a child SHALL be strictly less than on its parent, and SHALL NOT be negative.
- **S-DL-4.** A mandate with remaining delegation depth `0` SHALL NOT be used to issue a child mandate.
- **S-DL-5.** Child expiry SHALL be no later than parent expiry. Child `not_before` SHALL be no earlier than parent `not_before`.
- **S-DL-6.** Child scope, communication set, execution set, budget and constraints SHALL be no broader than the parent (equal or subset / equal or tighter). Equality on these axes is allowed. Adding a permission, destination, action, budget headroom or loosened constraint is **amplification** and SHALL be rejected. These axes are not required to become strictly narrower on every delegation.
- **S-DL-7.** The full delegation chain from the originating mandate to the acting mandate SHALL be available to the decision and evidence planes.
- **S-DL-8.** Revoking any ancestor mandate SHALL invalidate descendants.

---

## 7. Communication authority

Agents must have **explicit** authority to communicate with external agents, MCP servers, A2A endpoints, APIs and network destinations.

Unknown or unauthorized communication paths **must fail closed**.

### Requirements

- **S-CA-1.** A mandate SHALL enumerate allowed communication classes and destinations (or an explicit, bounded pattern language defined in architecture). Wildcards that mean “any destination” are forbidden unless a decision later approves a narrowly defined exception.
- **S-CA-2.** A request to communicate with a destination not positively matched SHALL be denied.
- **S-CA-3.** Communication authority is distinct from execution authority. Permission to call an endpoint is not permission to perform every action that endpoint exposes.
- **S-CA-4.** MCP servers, A2A endpoints, HTTP APIs and network destinations are all communication targets subject to this section.

---

## 8. Execution authority

Every requested action must be **independently authorized**.

Gateway authentication alone is **not** sufficient.

MCP, A2A, API gateways and agent runtimes **must** be treated as potentially compromised or untrusted enforcement environments.

### Requirements

- **S-EA-1.** Each requested action SHALL produce a distinct decision (allow, deny, or pending-approval) from the authority plane.
- **S-EA-2.** The system SHALL treat caller authentication, gateway session, and runtime assertion as untrusted inputs to be verified against the mandate and policy.
- **S-EA-3.** An allow decision SHALL bind mission, principal, agent, mandate, action, and decision time.
- **S-EA-4.** Replay of a previous allow decision SHALL NOT authorize a new action unless the mandate and policy explicitly grant a defined multi-use capability that is still valid.
- **S-EA-5.** Adapters MAY cache denials; they MUST NOT treat a cached allow as valid beyond the decision’s stated validity and revocation checks required by architecture.

---

## 9. Independent evidence plane

Agent runtimes and gateways must not be the sole source of truth.

The system shall record externally at least:

- original mission
- principal
- agent identity
- delegation chain
- authority decision
- approvals
- tool / API / payment call
- result or side effect
- revocation
- containment action

Evidence should support hash chaining, signing and immutable/WORM storage.

### Requirements

- **S-EV-1.** Evidence records SHALL be written by the authority/evidence plane, not solely by the runtime that performed the act.
- **S-EV-2.** The evidence log SHALL be append-only and hash-chained.
- **S-EV-3.** Each record SHALL include a type, timestamp, subject identifiers, payload hash, previous-record hash, and a signature or a recorded path to a signature.
- **S-EV-4.** Runtime-supplied “we did X” statements MAY be attached as claims; they SHALL NOT be the only evidence that X was authorized.
- **S-EV-5.** Storage MAY begin in PostgreSQL. Architecture MAY later add WORM/object-lock backends without changing record semantics.
- **S-EV-6.** Evidence MUST be sufficient to answer: who acted, under which mission, from which source, through which chain, what was decided, what was attempted, what was observed, and what was revoked.

---

## 10. Out-of-band control

The authority plane must support revocation **independent of the agent runtime**.

Revocation may include: delegation tokens, MCP credentials, payment authority, child-agent authority, active missions, and network access.

### Requirements

- **S-OB-1.** Revocation and containment SHALL be executable through the authority plane without a callback succeeding on the agent runtime.
- **S-OB-2.** After revocation, subsequent decisions for the revoked object SHALL deny.
- **S-OB-3.** Revocation of a mandate SHALL cascade to derived mandates, issued adapter credentials, and spend authority derived from it.
- **S-OB-4.** Containment actions SHALL themselves be evidence-recorded.
- **S-OB-5.** Adapters SHOULD push revocation to external systems (IdP sessions, MCP credentials, payment holds, network policy) but the authority-plane decision MUST NOT wait on those pushes to treat the mandate as revoked.

---

## 11. Financial authority

Agent Authority decides **whether an agent may spend**.

Payment rails are **adapters**. Potential adapters include Stripe, Visa, Mastercard, SEPA, x402 and other machine-payment systems.

The core product **owns authorization, not issuing**.

### Requirements

- **S-FA-1.** A mandate MAY include a budget: currency or unit, amount, remaining amount, and spend constraints.
- **S-FA-2.** A spend attempt SHALL require an independent execution decision against financial authority.
- **S-FA-3.** The core system MUST NOT issue payment instruments, hold customer card PAN data, or settle payments itself.
- **S-FA-4.** Adapters MAY execute an already-authorized spend instruction. They MUST NOT create authority.
- **S-FA-5.** Exhausted or revoked budget SHALL fail closed.

---

## 12. Counterparty trust

The system should eventually verify not only whether *our* agent may act, but whether the **external party or external agent** is trusted.

KYA (Know Your Agent) and counterparty credentials are **future extensions**.

### Requirements

- **S-CT-1.** The mandate and decision model SHALL leave an explicit extension point for counterparty evaluation.
- **S-CT-2.** Until a roadmap phase implements counterparty trust, decisions MAY proceed without it; they MUST NOT pretend a counterparty was verified.
- **S-CT-3.** When implemented, failure to verify a required counterparty SHALL fail closed.

---

## 13. Registry federation

Do **not** build a proprietary agent registry as the end product.

Integrate with external registries and provide portable trust and authority across them.

### Requirements

- **S-RF-1.** Agent discovery and directory services are adapters, not the core product.
- **S-RF-2.** Mandates and evidence MUST remain meaningful when the agent is listed in more than one external registry, or in none.
- **S-RF-3.** The system MAY cache registry attestations. It MUST NOT require exclusive registration in an Agent Authority registry to function.

---

## 14. Portable agent mandate

The mandate is the core portable authority object.

### 14.1 Required fields

| Field | Purpose |
| --- | --- |
| `principal` | Who granted / is accountable |
| `agent` | Who holds the mandate |
| `mission` | Why the authority exists now |
| `scope` | What class of work is in bounds |
| `constraints` | Tightening limits (time, data, geo, rate, etc.) |
| `budget` | Financial authority, if any |
| `expiry` | Inclusive end of validity |
| `delegation_depth` | Remaining allowed child hops |
| `approval_requirements` | When a human or external approval is required |
| `evidence_requirements` | What must be recorded for acts under this mandate |
| `authority_source` | Traceable origin |

### 14.2 Additional required fields (normative)

To make the object implementable and non-amplifying, a mandate SHALL also include:

| Field | Purpose |
| --- | --- |
| `mandate_id` | Stable unique identifier |
| `version` | Mandate schema version |
| `parent_mandate_id` | Null for originating mandates |
| `not_before` | Start of validity |
| `communication_authority` | Allowed communication targets |
| `execution_authority` | Allowed actions |
| `organization_id` | Tenancy boundary |
| `issued_at` | Issuance time |
| `issuer` | Authority-plane issuer identity |

### 14.3 Authority axes (attenuation)

A child is an attenuation iff it is **no broader** than its parent on every axis below. **Equality is allowed** on an axis unless that axis explicitly requires a strict reduction. The only axis that always requires a strict reduction is `delegation_depth`. Privilege amplification (any axis becoming broader) is forbidden. A child need not become strictly narrower on every axis.

A child is an attenuation iff all of the following hold:

1. `scope` ⊆ parent `scope`
2. `communication_authority` ⊆ parent `communication_authority`
3. `execution_authority` ⊆ parent `execution_authority`
4. every constraint is equal or tighter
5. budget ≤ remaining parent budget, in the same unit, with no extra spend category
6. `expiry` ≤ parent `expiry`
7. `not_before` ≥ parent `not_before`
8. `delegation_depth` < parent `delegation_depth`
9. `approval_requirements` are equal or stricter
10. `evidence_requirements` are equal or stricter
11. `mission` is the parent mission or a recorded sub-mission approved under that parent mission
12. `authority_source` is the parent source or a source that does not widen the grant

### 14.4 Portability

- **S-PM-1.** A mandate SHALL be representable as a vendor-neutral, machine-readable document.
- **S-PM-2.** Phase 1 MAY persist the logical model only. Before any external adapter relies on a portable mandate, a **separate accepted decision** SHALL define the mandate wire format and the cryptographic signing profile. This specification does not select that format or profile.
- **S-PM-3.** Verification of a mandate MUST NOT require a proprietary runtime.

---

## 15. Decision semantics

A decision request evaluates one proposed **communication**, **execution**, **delegation**, **spend**, or **control** act.

Possible results:

| Result | Meaning |
| --- | --- |
| `allow` | Authority exists now; evidence must be recorded. |
| `deny` | Authority does not exist or cannot be established. |
| `pending_approval` | Mandate requires an approval that has not yet been granted. |

Rules:

- **S-DC-1.** Default is deny.
- **S-DC-2.** `allow` requires valid identity binding, approved mission, unexpired non-revoked mandate, matching communication/execution authority, satisfied constraints, satisfied approvals, and remaining budget if spend is involved.
- **S-DC-3.** The decision API is the enforcement interface. Adapters ask it; they do not replace it.

---

## 16. Target integrations

The product SHALL be designed to integrate with, not replace:

- OAuth 2.x / OIDC
- Entra ID
- Okta
- SPIFFE/SPIRE
- MCP
- A2A
- REST/OpenAPI
- enterprise policy systems
- payment rails (Stripe, card networks, SEPA, x402, others)
- cloud agent registries

An integration is in scope only when its roadmap phase is approved.

---

## 17. Non-functional requirements

- **S-NF-1.** Fail closed on uncertainty, timeout of required control-plane state, or conflicting evidence.
- **S-NF-2.** Control-plane APIs SHALL be observable via OpenTelemetry.
- **S-NF-3.** Design SHALL be deployable as containers and on Kubernetes.
- **S-NF-4.** The evidence log SHALL support later WORM/immutable storage without semantic change.
- **S-NF-5.** The system SHALL be multi-tenant at the organization boundary.
- **S-NF-6.** Documentation and code identifiers SHALL be English. Product copy may later add locales without changing mandate semantics.
- **S-NF-7.** No compliance certification (EU AI Act, eIDAS, PCI-DSS, etc.) is claimed by this specification. Design SHOULD remain compatible with regulated European deployments (auditability, data-residency deployment, vendor neutrality).

---

## 18. Out of scope (unless a later approved decision adds them)

- Agent/model runtime, planner, memory or tool host
- MCP or API gateway as a product
- Identity provider, HR directory or endpoint agent
- Payment issuing, acquiring, or card-PAN processing
- A proprietary global agent directory as the product
- Automated implementation of market-watch findings
- End-user GUI (Phase 7 may start as API/workflow only)

---

## 19. Document control

Changes to this specification that alter requirements require an entry in `docs/DECISIONS.md` and a matching roadmap adjustment if they change phase scope.
