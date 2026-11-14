# MusicOps — Trusted Private-Draft Execution Bridge

**Status: implemented and tested in the existing draft PR #53 (2026-11-14); not deployed or enabled.**
Tracking: https://github.com/marko-durasic/balkazar.github.io/issues/41

## Goal

Marko reports one confirmed gig. MusicOps prepares candidates; a separately authenticated runner checks standing owner approval, gig facts, rights, duplicate posts and existing planner state. It submits only a **private Metricool draft**, then records a provider-verified receipt. Later public scheduling and other event channels require different permissions.

## Components

1. **Human-only owner issuer** (`cmd/musicops-owner`): create Ed25519 keypair, review and sign a bounded draft-only grant, revoke grant IDs. Keys must be kept in a human-controlled OS identity or hardware store **inaccessible to Desktop Commander and AI agents**. A TTY confirmation is only a UX prompt, not authentication.
2. **Signed policy and checks** (`draft_grant.go`): pinned owner key, revocation, Balkazar brand, specific gig/scope, approved networks and media, expiry, max drafts and zero spending.
3. **Durable bridge** (`bridge.go`, `campaign_store.go`): verify booking and owner grant, fetch live brand/planner, atomically record a dispatching intent **before** the single private-draft create, then re-read the provider to confirm the UUID/status. An ambiguous result remains blocked; reconciliation **never** creates a second draft. Reschedules and cancellations fail closed.
4. **Official MCP provider** (`mcp/clientprovider`): official Go MCP SDK v1.8.0, pinned hosted endpoint `https://ai.metricool.com/mcp`, injected OAuth handler or pre-installed, operator-reviewed OAuth-to-stdio proxy. Only three allowlisted tools are used (brand, scheduled posts, draft creation). If the remote tool schema cannot prove `draft=true` and `autoPublish=false`, the bridge **refuses** any create.
5. **Isolated runner** (`mcp/cmd/musicops-metricool-bridge`): operator-owned configuration and state, protected files, pinned owner public key/revocations, candidate JSON only from a specified inbox. OAuth tokens remain in the isolated runner identity. No key, credential or account secret may be passed as an LLM parameter.

## Security boundary

- AI agents may submit draft candidates but cannot issue/edit grants or access signing material and OAuth tokens.
- The owner signs a narrowly defined, revocable permission once. A human-facing approval issuer must run outside all agent-accessible devices/accounts.
- A dedicated service identity runs the bridge, enforces permissions, handles provider credentials and writes receipts. It MUST NOT expose a generic shell, arbitrary MCP tool proxy or secret material to the AI.
- **Current laptop Desktop Commander runs as ddjura. Do not deploy the owner signing key or production OAuth cache under that same account.** The prototype is in an isolated temporary worktree with no real credentials.
- The bridge uses a private flock-protected campaign store, writes a reservation before the outbound call and never retries an uncertain request. Even a cryptographically valid grant cannot authorize public publication: there is no publish method.
- Real provider responses are read by the trusted MCP adapter itself, not copied through chat from an untrusted file.

## Zero-cost provider route

Official Metricool hosted MCP: https://ai.metricool.com/mcp . Metricool says MCP works on Free, subject to free posting limits. The separate direct HTTP API requires Advanced/Custom. Use the hosted MCP and existing subscriptions; do not provision AWS instances, paid API keys or a second publishing platform by default.

Sources:
- https://help.metricool.com/how-to-connect-metricools-mcp-eqp9h
- https://help.metricool.com/mcp-limits-and-plan-requirements-h72jg
- https://help.metricool.com/mcp-vs-api-access-what-is-the-difference-5y3ib
- https://github.com/modelcontextprotocol/go-sdk

The user's existing Metricool ChatGPT connection is separate from an isolated local OAuth client. A new bridge identity requires one normal browser consent if deployed. This PR does NOT log in or store tokens. If the hosted MCP lacks explicit private-draft fields, refuse its scheduling tool; do not assume it defaults to drafts.

## Validation and remaining deployment steps

Local Go tests and GitHub CI exercise the core + MCP adapter with **mock providers**. Tests cover forged/revoked approvals, duplicate prevention, one-call-only semantics, crash/timeout ambiguity, readback verification, source drift/cancellation, protected files and the refusal of scheduling-only MCP write schemas. No test submits a real post.

Deployment is intentionally blocked on these prerequisites:
1. Establish an owner-only approval/signing identity inaccessible to the agent (or use a hardware-backed authorization mechanism).
2. Create a dedicated restricted executor identity on existing hardware/host, store protected config and grant policy, and complete the owner's one-time Metricool OAuth consent.
3. Discover the real hosted MCP tool schema and verify it can create a guaranteed private draft. If not, fail closed and choose a different supported draft method without upgrading a plan automatically.
4. With explicit authorization for **one new draft**, run one live pilot: recheck planned posts, create once, verify the stable UUID and `draft=true`/`autoPublish=false`, and demonstrate revocation. Do not modify the existing November 24 crowd-photo draft.
5. Wire the single-message gig intake to the executor's bounded inbox. Do not ask the owner to copy JSON, execute commands or supervise routine retriable work once the setup is complete.

**Until those steps are complete, this is an offline-tested code implementation, NOT an active autonomous publisher.**
