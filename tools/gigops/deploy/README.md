# MusicOps cloud dry-run worker

**Status:** implementation for existing [MusicOps PR #53](https://github.com/marko-durasic/balkazar.github.io/pull/53). This component performs no external requests, OAuth, event submission, social publication, media editing, or provider actions.

## Runtime and safety contract

The Go stdlib-only `cmd/musicops-cloud-worker` executable runs a **bounded single iteration**. A persistent systemd **user timer** invokes it every five minutes with catch-up after reboot; no laptop connection is required.

State defaults to an operator-configured private directory; for this cloud pilot the service uses `%h/.local/state/balkazar-musicops`. It only reads an optional existing `campaigns.json` from our typed GigOps campaign store and generates private `queue.json` and `heartbeat.json` files. Private permissions are required (directory 0700, regular JSON files 0600). File locks prevent concurrent runs and atomic fsync+rename protects queue/heartbeat. A corrupt queue fails closed; it is **never automatically overwritten**. Keys include gig ID, revision and job ID. Changes/cancellations mark prior keys as superseded. Jobs with incomplete dependencies are **waiting**, and external/physical tasks are **blocked/human-only**. Nothing runs automatically just because it is marked private-ready.

`heartbeat.json` includes checked_at, queue generation, counts, zero public writes, zero provider calls, and zero jobs executed. It is a health signal, **not proof of completed marketing work**.

## Deployment to an already approved cloud VM

Deploy under a single non-root user with its persistent systemd user manager enabled. Never install as an OAuth-bearing trusted executor; that must be a separately secured identity and approval flow.

From a reviewed, pinned Git SHA:
1. Run `go test ./...` and `go vet ./...` in `tools/gigops`.
2. Build `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/musicops-dryrun-bin ./cmd/musicops-cloud-worker`.
3. Install the binary atomically at `%h/.local/bin/balkazar-musicops-cloud-worker`, with no credential files. Put `musicops-cloud-dryrun.service` and `musicops-cloud-dryrun.timer` under `%h/.config/systemd/user/` (the unit specifier `%h` is expanded by systemd).
4. Create `%h/.local/state/balkazar-musicops` with mode 0700. Do not put a git checkout, media, OAuth or owner keys inside it.
5. Under an existing linger-enabled user bus, `systemctl --user daemon-reload`, manually start the dry-run service once and verify its status/heartbeat. Then `systemctl --user enable --now musicops-cloud-dryrun.timer`.
6. Confirm a second manual service start preserves queue generation, that old revisions become superseded with a dry-run fixture, and that the timer is active. Verify a clean report with `balkazar-musicops-cloud-worker --mode=health --state-dir=... --max-age=15m`.

This user-service is **not** a high-availability architecture: a single host and EBS root filesystem can fail. The current dry-run has not implemented an off-host backup. Before enabling any real credentials or outbound writes, verify separate IAM/root trust boundary, a real off-host backup/restore, recovery after reboot, cost limits, and provider-specific permissions. Do not reboot a shared production host merely to test this single service.

## Observability and rollback

No public ports or network access are needed. The `RestrictAddressFamilies=AF_UNIX` sandbox blocks normal Internet socket families, and the code has no network calls. `NoNewPrivileges=true`, `UMask=0077`, memory and CPU limits, and systemd user isolation reduce blast radius. Verify actual systemd enforcement before claiming it is sandboxed; if an option is unsupported, **stop rather than silently removing security**.

Inspect `systemctl --user status musicops-cloud-dryrun.timer` and `systemctl --user status musicops-cloud-dryrun.service`; logs from `journalctl --user -u musicops-cloud-dryrun.service`. Check the private `heartbeat.json` via the health command rather than treating `active` timer status as proof of execution.

To roll back **only this new worker**, disable and stop its timer and service (`systemctl --user disable --now musicops-cloud-dryrun.timer`; `systemctl --user stop musicops-cloud-dryrun.service`). Retain private queue/heartbeat for review; don't delete or overwrite someone else's state. An intentionally stopped timer must never be described as an autonomous controller.

## Next gate

This iteration creates cloud durability and read-only status **only**. Next safely connect an authenticated intake path that can write the canonical `campaigns.json` (under the same file-lock policy), then verify queued work and rollback against two real confirmed gigs. Only after owner-approved secure trust separation should the separate Metricool private-draft executor be deployed.
