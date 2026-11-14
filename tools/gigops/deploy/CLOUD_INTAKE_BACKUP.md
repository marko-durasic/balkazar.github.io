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

## Legacy compact SSM backup (disabled; NOT recommended for growing gig history)

`cmd/musicops-cloud-backup` and `cloud_backup.go` construct a private, checksum-verified gzip+base64 bundle of exactly `campaigns.json`, `queue.json`, and `heartbeat.json`. Backups reject stale heartbeats, inconsistent campaign/queue revisions, unknown JSON fields, corrupt inputs and data exceeding the **Standard Parameter Store 4-KB cap**. An actual current-state preview was approximately 3.1 KB.

- `--mode=preview --state-dir ...` is read-only and returns only counts/hash/size. **No AWS call**.
- `--mode=upload-standard --state-dir ... --allow-standard-securestring` is an explicit **external** write through AWS CLI to one fixed dedicated SSM parameter in `us-east-1`, Type=`SecureString`, Tier=`Standard`, with a new numbered version each time and a readback of the exact version. It uses a short-lived protected CLI-input file, not a command-line value. The code never silently switches to Advanced/higher-throughput.
- `--mode=restore-version --version=N --restore-dir /ABS/NEW/DIR --allow-standard-securestring` decrypts a specific SSM version and validates it into a **new private directory**, never overwriting production state. A restored directory can run the no-network worker. Downloading an SSM parameter requires existing permissions.
- There is **no automatic backup timer enabled**. The existing AWS IAM role denied `ssm:PutParameter` for the dedicated backup path. Changes to IAM and any KMS/SSM billing permission must be separately reviewed before enabling. AWS Standard parameters and standard throughput are ordinarily free, but KMS requests may be chargeable beyond the account-wide free tier. The role currently cannot inspect high-throughput settings, so do **not** assume total zero cost without an independent billing check.
- The cloud role cannot enumerate S3 buckets. A versioned DuReef CAO bucket is defined in existing infrastructure-as-code, but its current state, permission scope, costs and availability must be verified before use. Do not escalate permissions from the agent.
- Restore simulations passed offline with checksum, corruption, stale/inconsistent source, independent target directory and next-run recovery. **This is not a real off-host disaster recovery test yet**.

## Required next infrastructure gate

**Superseded by the versioned S3 approach below.** Do not seek additional IAM access or turn on a scheduled backup for the 4-KB SSM parameter. Inspect the existing DuReef versioned CAO bucket and least-privilege object-prefix permissions instead; approve any incremental S3 storage/request costs before an actual upload.

The current dry-run worker stays network-isolated and without social/OAuth/LINE write permissions. The owner signing authority and a real Metricool executor remain separate security projects. Do not activate either as a side effect of the cloud intake.

## Quality gates

- Cloud tests cover new gig, exact replay, edited gig, stale revision, cancellation, wrong artist, extra fields, symlink/permissions and concurrent lock protection.
- Backup tests cover prepare, checksum/tamper rejection, stale/inconsistent state, Standard size cap, replay/recovery, and refusal to overwrite existing directories.
- Mock AWS tests assert SecureString + Standard only, version-pinned readback, no Advanced tier and no automatic network calls during preview.
- PR remains reviewable and unmerged; deployed binaries are pinned to passing commits before any change to production.


## 2026-11-14 superseding backup decision — versioned private S3

**S3 is now the preferred future backup storage. The preceding SSM instructions describe a disabled prototype, not the next deployment action.** A synthetic capacity test with the real queue schema produced **3,901 encoded bytes at two gigs** and exceeded SSM Standard's 4-KB limit by three gigs. The new versioned object-backup variant prepared, verified, restored, and resumed six synthetic gigs without exceeding its 256-KB explicit cap.

DuReef's existing CDK `infra/lib/state-stack.ts` defines a private, versioned, S3-managed AES256-encrypted CAO backup bucket with public-access blocking, SSL enforcement, noncurrent-version 90-day retention, and no auto-delete. Existing `infra/lib/worker-stack.ts` describes narrowly scoped object permissions for a different role. **These source files do not establish that the current MusicOps interactive cloud role has access.** On November 14 it received `AccessDenied` for `s3:GetBucketVersioning`. Do not copy credentials or bypass IAM.

The new `cloud_backup.go` code supports `PrepareCloudObjectBackup`, `DecodeCloudObjectBackup`, and `RestoreCloudObjectBackup` (no network calls), alongside the existing legacy SSM functions. The `cmd/musicops-cloud-backup/s3.go` adapter is restricted to an existing deterministic DuReef CAO bucket in the authenticated account and one object key: `musicops/dryrun/cloud-state.json`. Before uploading, it checks bucket versioning; the put explicitly requests `AES256` and requires a stable S3 `VersionId`. Readback requests that exact version and confirms encryption and matching bytes. It does not create a bucket, key, new infrastructure, delete objects, or publish anything.

Future operator modes are `--mode=s3-preview` (read-only), `--mode=s3-upload --s3-bucket <existing-bucket> --allow-existing-s3` (one explicit write), and `--mode=s3-restore-version --s3-bucket <existing-bucket> --s3-version <exact-id> --restore-dir <fresh-path> --allow-existing-s3`. The upload/restore paths must remain blocked until least-privilege IAM, readback, lifecycle and real incremental request/storage costs have been reviewed and approved.

**Current status:** locally compiled and unit-tested with an in-memory/mock AWS CLI only. No S3 object upload or off-host restore is verified. No recurring backup timer, new paid service or IAM modification is authorized. Reusing an existing S3 bucket may still incur a small incremental storage/request cost; do not describe it as guaranteed free. Keep OAuth/signing credentials, photos, audio and personal chats out of this backup.

**Next engineering gate:** review the existing S3 bucket owner/versioning and account cost data via a read-only authorized role, then obtain only explicitly approved object-prefix permissions if justified. Make one versioned backup, read back the same version, restore into a separate private directory, and compare queue/campaign state. Enable at most daily backups only after this succeeds.
