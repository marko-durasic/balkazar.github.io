# GigOps production architecture: cloud-first, laptop-independent

**Decision (2026-11-14):** The owner's laptop is a **development/test environment only**. No production intake, campaign scheduling, event distribution, authorization check, provider OAuth session, persistent queue, or recovery task may require the laptop to be powered on, online, remotely accessible, or logged in.

**Current status:** Bridge implementation is in [draft PR #53](https://github.com/marko-durasic/balkazar.github.io/pull/53) and tested with fakes. It is **not yet deployed**, no standing signing key has been issued, and no real Metricool draft has been created. The cloud design below is a deployment contract, not a declaration of readiness.

## Reuse and availability

- Deploy a small **always-on control plane** to an **existing, approved cloud environment**. Prefer a bounded systemd service on existing capacity, **not** a new EC2 instance or another paid automation platform. If an existing scheduler is trustworthy and suitable, reuse it. GitHub Actions stays for code review, tests, reproducible artifacts, and optional non-critical reconciliation—not the sole always-on runtime.
- Store canonical Gig IDs, job states, deadlines, draft intent reservations, provider UUID receipts, errors and audit history **durably in cloud storage**. Initially a small locally persistent database or private ledger is reasonable; use atomic writes, locking, checksums and backup. The current JSON+flock proof-of-concept must be reviewed for safe multi-process/host failover before introducing a second worker. A lightweight SQLite transactional queue is a candidate migration when scheduling/parallelism grows; **do not silently fork the source of truth**.
- Never place original performance media/video or heavyweight FFmpeg transcoding on the low-capacity control plane. Keep originals in the existing authorized media archive, ingest metadata, generate edits in a bounded worker using approved subscription/compute quotas, and write results back to the designated media asset store. Protect rights, provenance and private recordings.
- The controller polls or receives confirmed gigs, generates eligible per-channel jobs, schedules due items, checks provider readback, and updates canonical receipts. Each job has one durable idempotency key, a state-machine transition, a deadline/lease or reconciliation state, and exception-only reports. Cancellation/reschedule must **reconcile existing posts**, not create another event.

## Separate the cloud agent from the trusted execution boundary

The existing AI-accessible cloud session **cannot automatically be considered trusted just because the OS has separate user accounts**. Before placing production OAuth tokens on any machine, examine its actual sudo/root, Docker/socket, remote-access, IAM/role, deployment and backup privileges. A process that can escalate to root, read the executor's secret files, replace its executable, or assume its IAM role is **not isolated** from the executor.

1. **Agent/orchestrator role:** may submit typed candidate jobs and read minimal status/receipts. It can run with the main cloud controller. It must not see the owner private signing key, OAuth tokens, privileged API credentials or broad shell execution on the trusted executor.
2. **Owner approval role:** has the signing key or hardware/managed approval authority **outside AI-accessible infrastructure**. One-time revocable standing grants can let approved draft tasks run even when the owner's phone and laptop are offline. Owner access is needed only to issue/revoke authority or resolve exceptions, not at every post.
3. **Trusted executor role:** processes only signed, scoped, unexpired and unrevoked instructions; independently fetches current provider state and enforces channel, campaign, media, time and budget limits. Owner-pinned trust configuration and provider OAuth storage must be protected from the agent and from unreviewed automated deployments. The code in `bridge.go` performs a draft-only, one-attempt transaction with readback; its remote deployment depends on achieving this **actual** isolation.
4. **Software supply-chain boundary:** merging code or changing config should not silently give an AI the authority to execute a new version with production credentials. Use reviewed releases, pinned artifacts, least-privilege deployment credentials and a separate owner-controlled approval for changes to trusted execution logic. No secrets in public GitHub, PR artifacts, ClickUp or model prompts.

If adequate isolation is not available on the existing VM, keep the cloud orchestrator on that VM and put the credential-bearing bridge in a **separately secured existing provider boundary** (or propose the lowest-cost permitted alternative for owner approval). Do not claim that an ordinary Linux service account on an agent-root-accessible host is a security boundary.

## Reliability, outages and cost

- **At boot:** start the cloud service with systemd/Docker restart policy as appropriate, a constrained user, no generic public listener, health checks and resource limits. Never execute arbitrary scripts or browser sessions sourced from untrusted gig data.
- **When the provider or network is down:** save the job status and last confirmed receipt, back off and reconcile; never auto-retry an ambiguous create. Send a consolidated exception when a true deadline is at risk. An observed draft is not a live social post.
- **When the host reboots:** restart the agent, load durable state, verify previous dispatch reservations against the provider, and resume only safe queued work. Use periodic backups and test an actual restore before claiming recovery.
- **Disk and resource controls:** define capped logs, queue retention, storage thresholds, memory/CPU limits and a watchdog before rollout. Do not use the controller host as a video archive.
- **Monitoring:** heartbeat, scheduler lag, pending approval, missed publication deadlines, provider auth failure, disk pressure and backup freshness; route genuinely actionable alerts to the existing LifeOps notification infrastructure instead of creating noisy parallel bots.
- **Cost:** reuse the existing paid-for host and tools; provision no new instance, paid API fallback, managed secret service, large storage tier, or always-on transcoder without an explicit cost/benefit review and permission. A no-surprise-bills hard guard applies.
- **Availability claim:** a single existing VM plus backups/restarts improves unattended reliability but remains a **single point of failure**. True high availability requires independent failure domains/replicas and tested failover, likely involving additional cost. Build for recovery first and measure missed scheduled work before recommending paid HA. Do not advertise 99.9% uptime without actual evidence.

## Deployment gates

**Gate A — cloud preflight (read-only):** inspect operating system, current workloads, resource/disk headroom, service manager, storage persistence, credential privileges, existing backups and current monthly costs. Record sensitive topology only in private operations notes, not this public repo.

**Gate B — cloud control plane:** deploy only the non-secret queue/state reader and dry-run job processor behind authenticated internal access. Verify reboots, restart, idempotency, backup/restore and a cancelled gig with a fake provider. No public writes.

**Gate C — trusted execution:** establish a genuinely isolated credential and signing trust boundary, perform the owner's one-time OAuth permission flow, discover the hosted provider schema and require explicit draft-only semantics; sign one narrowly scoped test grant. Keep the normal chat agent unable to issue it.

**Gate D — one live private draft:** with specific owner authorization and rights-cleared art, do one new private Metricool draft. Verify it exists as `draft=true` and `autoPublish=false`, UUID stable and no duplicate. Demonstrate revocation and recovery after an ambiguous response. Do not alter the existing planned crowd-photo draft.

**Gate E — fully autonomous approved routine:** intake one confirmed gig once; process approved listings and media steps without laptop connectivity, log receipts and exception-only blockers. Before widening channels or permissions, pass two real gigs including one changed/cancelled gig and a cloud restart/restore test.

**Stop conditions:** No isolated secrets, unresolved cloud privilege reachability, ambiguous provider draft capability, storage/backup failure, unexpected resource/billing growth, unsupported license/terms, or any duplicate/public publication during a private-draft test. Fail closed and keep existing production accounts unchanged.

**Important:** This is an architectural direction and a checklist of technical acceptance criteria. None of the gates grants permission to install a service, expose a public port, perform a paid operation, create signing keys, obtain OAuth access, merge this PR or publish social content.
