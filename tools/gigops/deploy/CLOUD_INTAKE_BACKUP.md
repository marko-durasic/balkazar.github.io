# Cloud GigOps intake and backup — operator runbook

**Status 2026-11-14:** Cloud dry-run timer remains active on existing DuReef host, with no provider calls or publishing. This PR adds a versioned, cloud-only private intake and a 4-KB-capped backup/restore implementation. **Off-host backup is NOT active**: the existing IAM role returns `AccessDeniedException` on the dedicated AWS Systems Manager Parameter Store write. No IAM broadening, paid AWS resources, SSM parameter, or OAuth enrollment was performed.

## One-message cloud intake

The existing ChatGPT-authenticated Remote Desktop Commander can submit a strict, private JSON candidate to the **cloud** under the existing `dureef` account. No laptop, listening port, webhook, browser, bearer token, or publicly available endpoint is required.

1. The agent reads the canonical gig booking/source, reconciles artist, date, venue, role and existing gig IDs; uncertain/held gigs stay private and blocked.
2. It creates a **private, mode 0600** file directly inside `~/.local/state/balkazar-musicops/inbox/`. The inbox is 0700 and under the same private state root.
3. The file schema is `{"intake_version":1,"expected_revision":0,"gig":{...}}`, where `gig` is the exact `GigIntake` type and `expected_revision=0` creates a new gig (or allows an exact idempotent replay). For edits/cancellation, the agent must read the current campaign revision and provide that revision. **Stale conflicting requests fail closed under the exclusive campaign file lock.**
4. The cloud command is `musicops-cloud-intake --state-dir ~/.local/state/balkazar-musicops --file /ABS/PATH/TO/INBOX/confirmed.json --apply`. Without `--apply`, it returns **only a read-only plan**. With `--apply`, it performs private campaign reconciliation, then updates the queue metadata immediately. The five-minute cloud timer safely picks up the change after a process interruption.
5. The output includes the gig ID, revision, change, private queue generation and blockers. `public_writes=0`, `provider_calls=0` and `executed_jobs=0`. A stored record is not a live listing or scheduled post.
6. **Human does not need to run shell commands**: the assistant can invoke the existing authorized cloud connector after receiving a confirmed gig. The connector itself is the current authentication boundary. It is not yet a dedicated narrow ChatGPT custom plugin; future direct remote MCP requires separate authorized deployment.

## Compact backup/restore

`cmd/musicops-cloud-backup` and `cloud_backup.go` construct a private, checksum-verified gzip+base64 bundle of exactly `campaigns.json`, `queue.json`, and `heartbeat.json`. Backups reject stale heartbeats, inconsistent campaign/queue revisions, unknown JSON fields, corrupt inputs and data exceeding the **Standard Parameter Store 4-KB cap**. An actual current-state preview was approximately 3.1 KB.

- `--mode=preview --state-dir ...` is read-only and returns only counts/hash/size. **No AWS call**.
- `--mode=upload-standard --state-dir ... --allow-standard-securestring` is an explicit **external** write through AWS CLI to one fixed dedicated SSM parameter in `us-east-1`, Type=`SecureString`, Tier=`Standard`, with a new numbered version each time and a readback of the exact version. It uses a short-lived protected CLI-input file, not a command-line value. The code never silently switches to Advanced/higher-throughput.
- `--mode=restore-version --version=N --restore-dir /ABS/NEW/DIR --allow-standard-securestring` decrypts a specific SSM version and validates it into a **new private directory**, never overwriting production state. A restored directory can run the no-network worker. Downloading an SSM parameter requires existing permissions.
- There is **no automatic backup timer enabled**. The existing AWS IAM role denied `ssm:PutParameter` for the dedicated backup path. Changes to IAM and any KMS/SSM billing permission must be separately reviewed before enabling. AWS Standard parameters and standard throughput are ordinarily free, but KMS requests may be chargeable beyond the account-wide free tier. The role currently cannot inspect high-throughput settings, so do **not** assume total zero cost without an independent billing check.
- The user-account role also cannot list S3 buckets, so don't improvise an unknown S3 target, reuse a credential-backup namespace, create a paid advanced parameter, or escalate IAM from the agent.
- Restore simulations passed offline with checksum, corruption, stale/inconsistent source, independent target directory and next-run recovery. **This is not a real off-host disaster recovery test yet**.

## Required next infrastructure gate

Read-only audit the current role and backing-store billing, then obtain a **minimal approved policy** for the *single* dedicated MusicOps SSM parameter to read/write Standard SecureString and use the existing AWS-managed SSM KMS key for Encrypt/Decrypt. Do not create new KMS keys or paid tiers. Limit backup to at most daily, fail closed above 4 KB, and verify a specific-version off-host restore before calling it production-ready.

The current dry-run worker stays network-isolated and without social/OAuth/LINE write permissions. The owner signing authority and a real Metricool executor remain separate security projects. Do not activate either as a side effect of the cloud intake.

## Quality gates

- Cloud tests cover new gig, exact replay, edited gig, stale revision, cancellation, wrong artist, extra fields, symlink/permissions and concurrent lock protection.
- Backup tests cover prepare, checksum/tamper rejection, stale/inconsistent state, Standard size cap, replay/recovery, and refusal to overwrite existing directories.
- Mock AWS tests assert SecureString + Standard only, version-pinned readback, no Advanced tier and no automatic network calls during preview.
- PR remains reviewable and unmerged; deployed binaries are pinned to passing commits before any change to production.
