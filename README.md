# Agent Authority

Independent authority and evidence infrastructure for autonomous AI.

Project codename: **European AgentOS**.

Agent Authority is vendor-neutral infrastructure for **authority, delegation, trust and evidence**. It answers not only *what an agent may do*, but *why it may do it right now*, and it records that answer independently of the agent runtime.

It is **not** an agent framework, MCP gateway, generic agent-security product, IAM-for-agents system, or AI credit card.

## Positioning

**Embrace existing standards and vendors; own the authority enforcement and evidence layer.**

Identity, payment rails, agent runtimes, MCP servers, A2A endpoints and cloud registries already exist. Agent Authority sits beside them as an independent plane that:

- traces authority to an original source
- binds every autonomous action to an approved mission
- issues portable, attenuating agent mandates
- authorizes communication and execution independently of gateways
- records hash-chained evidence outside the runtime
- revokes and contains authority out of band

## Core concepts

| Concept | Role |
| --- | --- |
| **Identity** | Integration layer for Entra ID, Okta, SPIFFE, workload identity and custom IdPs. Not the product moat. |
| **Authority source** | Traceable origin of permission: human approval, role, policy, contract, mandate or credential. |
| **Mission** | Why authority exists *now*. No autonomous action without an approved mission. |
| **Delegation** | Child agents may receive authority only by attenuation: no broader than the parent; equality allowed except where an axis must strictly decrease (`delegation_depth`). Privilege amplification is forbidden. |
| **Communication authority** | Explicit permission to reach external agents, MCP servers, A2A endpoints, APIs and destinations. Unknown paths fail closed. |
| **Execution authority** | Every requested action is independently authorized. Gateway authentication is not authorization. |
| **Evidence plane** | Independent, hash-chained record of missions, decisions, calls, results, revocations and containment. |
| **Out-of-band control** | Revocation and containment without cooperation from the agent runtime. |
| **Financial authority** | Decide whether an agent may spend. Payment rails are adapters. |
| **Portable agent mandate** | Machine-readable authority object that travels with the agent. |

See [docs/SPECIFICATION.md](docs/SPECIFICATION.md) for normative definitions.

## Documentation

| Document | Authority |
| --- | --- |
| [AGENTS.md](AGENTS.md) | Engineering-agent contract for this repository |
| [docs/SPECIFICATION.md](docs/SPECIFICATION.md) | Authoritative product specification |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Architecture and data model |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Phases and acceptance criteria |
| [docs/DECISIONS.md](docs/DECISIONS.md) | Approved architecture and product decisions |
| [docs/MARKET-WATCH.md](docs/MARKET-WATCH.md) | Informational landscape only |

**`docs/SPECIFICATION.md` is authoritative.** Architecture explains how the specification is realized. The roadmap says what may be built, and when. Decisions record why. Market-watch findings must never be implemented until they become an approved decision or roadmap item.

## Implementation stack

Approved for implementation phases (not started in Phase 0):

- Go backend
- PostgreSQL
- Policy engine initially compatible with OPA/Rego
- Redis only when a decision justifies it
- OpenTelemetry
- Docker, Kubernetes-ready
- Hash-chained append-only evidence log
- Python and TypeScript SDKs later

## Current status

**Phase 0 — Repository and specification foundation** is `locked`.

**Phase 1 — Core authority domain model** is `locked`.

**Phase 2 — Mission and delegation engine** is `approved` and is the current implementation phase.

## Development discipline

Work on a **feature branch** and open a **pull request** to `main`. Do not commit or push to `main`. Merged branches are deleted automatically.

Every implementation phase must:

1. Read `AGENTS.md`
2. Read the specification, architecture and roadmap
3. Implement only the currently approved phase
4. Add tests
5. Run linting and tests
6. Produce `reports/PHASE-N-VERIFICATION-REPORT.md`
7. Stop
8. Not proceed to the next phase without explicit approval
