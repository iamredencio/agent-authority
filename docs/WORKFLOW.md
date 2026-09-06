# Engineering Workflow

**Purpose:** eliminate manual prompt transfer between ChatGPT and the implementation agent while preserving human approval and repository governance.

## Operating model

| Role | Responsibility |
| --- | --- |
| Human | Approves phases, decisions, merges and scope changes |
| ChatGPT | Product owner / architect / reviewer; translates approved work into GitHub work orders and reviews delivery |
| GitHub Issue | Approved engineering work order and handoff boundary |
| Repository docs | Authoritative product and engineering state |
| Cursor / engineering agent | Implements the approved work order according to `AGENTS.md` |
| Pull request | Delivery and review boundary |
| Verification report | Acceptance evidence for a phase |

## Source-of-truth rule

The repository remains authoritative. A GitHub Issue does not override `docs/SPECIFICATION.md`, `docs/DECISIONS.md`, `docs/ROADMAP.md`, `docs/ARCHITECTURE.md`, or `AGENTS.md`.

A work order is valid only when its requested work is already permitted by the repository state and has explicit human approval.

## Standard flow

1. Human approves a phase or bounded engineering task in ChatGPT.
2. ChatGPT checks the current repository state.
3. ChatGPT creates a GitHub Issue using the approved-work template.
4. The Issue records scope, non-goals, acceptance criteria, required verification, and the human approval statement.
5. Cursor reads `AGENTS.md`, authoritative docs, and the referenced GitHub Issue.
6. Cursor creates/uses the required feature branch and implements only the work order.
7. Cursor runs tests/lint and creates the required verification report.
8. Cursor opens a PR that links the Issue and verification report.
9. Human asks ChatGPT to review the PR/repository.
10. ChatGPT reviews code, docs, tests and verification evidence against the Issue and roadmap.
11. Human decides whether to merge/accept and whether the phase may become `locked`.
12. The next phase remains blocked until separately approved.

## Work-order contract

An approved engineering Issue SHOULD contain:

- phase/task identifier
- explicit approval statement
- objective
- authoritative documents to read
- in-scope work
- explicit non-goals
- invariants and constraints
- acceptance criteria
- tests/lint requirements
- required verification report
- stop condition

The Issue MUST NOT silently widen the roadmap. If the requested work conflicts with repository docs, the engineering agent stops and reports the conflict.

## Cursor handoff

The normal Cursor instruction should be minimal:

> Implement the approved GitHub Issue for the current phase according to `AGENTS.md` and the authoritative repository docs. Do not widen scope. Stop after the required verification report and PR are ready.

If multiple approved Issues exist, the human or Issue reference must identify exactly one. Cursor must not choose a backlog item autonomously.

## PR contract

A phase PR SHOULD:

- link the work-order Issue (`Closes #N` only when merging should close it)
- identify the phase
- summarize implementation
- list tests/lint run
- link `reports/PHASE-N-VERIFICATION-REPORT.md`
- state that the next phase was not started
- call out deviations or unresolved risks

## Review contract

ChatGPT review checks at minimum:

1. Issue scope vs diff.
2. Roadmap phase vs implemented functionality.
3. Specification and decision compliance.
4. Acceptance criteria.
5. Tests and lint evidence.
6. Verification report accuracy.
7. No future-phase scaffolding or speculative market-watch work.
8. Whether the phase is safe to mark `locked`.

## Market watch

`docs/MARKET-WATCH.md` never creates a work order automatically. A market finding must first be promoted through an accepted decision and/or approved roadmap amendment, then explicitly approved by a human.

## Automation boundary

Automation may create Issues, branches, PRs, comments, reports and review handoffs. It must not bypass the human gates defined by `AGENTS.md` and `docs/ROADMAP.md`.

In particular, automation must not:

- approve a new phase on behalf of the human
- merge a PR without explicit human instruction
- mark a phase `locked` merely because CI passes
- promote market-watch findings into implementation
- start Phase N+1 after Phase N finishes
