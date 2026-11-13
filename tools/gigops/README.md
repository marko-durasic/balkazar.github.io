# MusicOps — local Songkick pilot (review-only)

[Engineering issue #41](https://github.com/marko-durasic/balkazar.github.io/issues/41) · [Event-listings ClickUp](https://app.clickup.com/t/z91464juf6) · [Draft PR #53](https://github.com/marko-durasic/balkazar.github.io/pull/53)

**NOT a live Songkick publisher.** These components cannot create, edit, scrape, or publish Songkick events. They are an authenticated-browser bootstrap and a read-only, self-hostable coordination foundation. No AWS VM, external server, paid tool, or credentials are required to test them.

## Existing pieces

- `tools/giglist/` already produces per-gig bilingual event metadata from the website's `MusicEvent` JSON-LD and CI read-only `giglist-review` artifacts.
- `tools/gigops/`: minimal dependency-free Go planner, source URL/fingerprint validator and **private local** idempotent listing ledger. It records public links only after you verify them, and NEVER submits to platforms.
- `tools/gigops/mcp/`: official Go MCP SDK-based **stdio-only** plugin with THREE read-only tools: `musicops_list_gig_plans`, `musicops_get_gig_plan`, and `musicops_preview_gig_intake`. It cannot access arbitrary paths through tool inputs or browse on its own. Local MCP client must support running the server.
- `tools/gigops/browser/`: The login launcher now opens a **regular system Chrome/Brave**, using an isolated native browser profile outside the public repo. Google may reject sign-in through Playwright-controlled Chromium; do not retry an automated OAuth login or try stealth/bypass. The launcher does not inspect pages, click, upload or publish.

## Easiest Linux SSH setup — ONE line to paste

If you are on your phone connected by SSH to your **Linux laptop**, copy this single command line into the laptop's SSH terminal. It downloads the [reviewable, pinned setup script](https://github.com/marko-durasic/balkazar.github.io/blob/ef00ec1d488d80f75d5456c8ae22580f372fa6cb/tools/gigops/setup-linux-ssh.sh) to `~/musicops-setup.sh` and runs it **only after** a successful download:

```bash
curl -fsSL https://raw.githubusercontent.com/marko-durasic/balkazar.github.io/ef00ec1d488d80f75d5456c8ae22580f372fa6cb/tools/gigops/setup-linux-ssh.sh -o "$HOME/musicops-setup.sh" && bash "$HOME/musicops-setup.sh"
```

The setup script checks Linux/Node.js 22+/Git/npm, clones Balkazar to `~/src/balkazar.github.io` if missing, safely fetches and checks out PR #53 into `musicops-pr53`, installs Playwright for future isolated testing, and runs **offline** safety tests. **For Google/Songkick sign-in, use installed regular Chrome/Brave, not Playwright-controlled Chromium.** It refuses to overwrite an existing dirty checkout or merge diverged history. It **does not launch a graphical browser, log in to Songkick, publish events or change AWS resources**. You can inspect `~/musicops-setup.sh` before running if preferred.

If you already have the repository in **another directory**, set its absolute path for the second command instead:

```bash
MUSICOPS_REPO_DIR="$HOME/path/to/existing/balkazar.github.io" bash "$HOME/musicops-setup.sh"
```

When you later have access to the laptop's graphical desktop, run `cd ~/src/balkazar.github.io/tools/gigops/browser && npm run login:songkick` (replace the path if you chose another checkout). The normal Songkick password/MFA stays within that browser.

## Start here — complete Linux laptop checkout and Songkick login

**The following commands include cloning GitHub and checking out draft PR #53.** They work for a fresh checkout and do not run any Songkick publishing actions. These longer checkout commands can run in an SSH terminal too, but the final `npm run login:songkick` needs a visible graphical desktop session (not an SSH-only terminal or CI).

If you do **not** already have this Git repository on your laptop, run **all** of:

```bash
mkdir -p "$HOME/src"
cd "$HOME/src"
git clone https://github.com/marko-durasic/balkazar.github.io.git
cd balkazar.github.io

# Download the precise HEAD of GitHub pull request #53.
git fetch origin pull/53/head
# Create and switch to a local branch tracking that fetched PR snapshot.
git switch -c musicops-pr53 FETCH_HEAD
git status --short

# Requires Node.js 22+ and npm.
node --version
npm --version

cd tools/gigops/browser
npm install
npx playwright install chromium
npm run login:songkick
```

If this repository is **already cloned elsewhere**, do **not** clone it again. Instead:

```bash
cd /path/to/YOUR/existing/balkazar.github.io
git status --short
# If that prints modified/untracked work, save it safely first. Do not discard it.
git fetch origin pull/53/head
git switch -c musicops-pr53 FETCH_HEAD
cd tools/gigops/browser
npm install
npx playwright install chromium
npm run login:songkick
```

If you have already created the `musicops-pr53` branch and want the newest PR changes, use `git switch musicops-pr53 && git fetch origin pull/53/head && git merge --ff-only FETCH_HEAD` (only when you have no uncommitted changes).

If `node --version` is below v22 or `npm` is missing, install Node 22+ using your existing version manager or a trusted Node distribution before proceeding. If the Playwright browser reports missing Linux system libraries, consult the Playwright installation instructions and run `npx playwright install --with-deps chromium` only after reviewing the packages it proposes. This may require `sudo`; don't run arbitrary scripts with elevated permissions.

A normal installed Chrome or Brave window opens `https://tourbox.songkick.com/` in a separate native profile. On Xfce/Chrome Remote Desktop, this should be visible inside the remote desktop session. If the script cannot find a native browser, open the existing **Brave Web Browser desktop shortcut** and enter the Tourbox URL manually. You manually sign in and handle MFA. In `Your artists`, check whether **Balkazar** already exists; if absent, use official `Add artist` and Songkick's requested relationship verification. Do not assume access because the personal Songkick login exists. Verify the Songkick artist page and the actual venue record for Xiangti Avenue Plaza; inspect December 3 drafts/live listings before considering an event submission.

Close the native browser window normally when finished; it saves the private session locally, subject to Songkick expiration. The default directory is the user's application-data directory (not this repo); to override use `MUSICOPS_NATIVE_BROWSER_PROFILE` with an **absolute private path**. Browser cookies/passwords/MFA/session exports must never go in GitHub, ChatGPT, a public URL or GitHub Actions. Google OAuth should not be performed through automated Chromium, and the native profile is NOT silently shared with Playwright. Future automated publishing needs separate platform authorization.

**Songkick permission:** The current terms restrict certain automated accessing, monitoring and commercial uses without written permission. Do not enable unattended Playwright crawling, editing or publishing until authorization is confirmed. This script is deliberately an interactive human login bootstrap. For the one-time December 3 gig, use the legitimate human-supervised Tourbox event form after checking artist/venue and duplication; no fake market ticket links.

## See plans and record *verified* public listing URLs

From repository root:

```sh
mkdir -p "$HOME/.config/balkazar-musicops"
python3 tools/giglist/review_bundle.py --out "$HOME/.config/balkazar-musicops/review-bundle.json"
cd tools/gigops
go run ./cmd/musicops plan --bundle "$HOME/.config/balkazar-musicops/review-bundle.json"
```

For an event that is truly published and verified, record its existing URL in a **private on-device** ledger; e.g. the previously published Bandsintown show:

```sh
go run ./cmd/musicops record-live \
  --bundle "$HOME/.config/balkazar-musicops/review-bundle.json" \
  --source 'https://balkazar.org/events/2026-12-03-european-christmas-market/' \
  --platform bandsintown \
  --listing-url 'https://www.bandsintown.com/e/109070616-balkazar-at-xiangti-avenue-plaza' \
  --confirmed-human-verification
```

This action writes **only the local ledger**. It rejects unapproved listing domains, conflicting live event URLs, and changes to a previously recorded event fingerprint. No network access, publication, or auto-notification occurs. The private ledger defaults to the user-config directory; do not store it in the git repository.

## Read-only local MCP plugin

Requires Go 1.25+ (the core CLI uses Go 1.23+) and the pinned official MCP Go SDK, installed on your own computer:

```sh
cd tools/gigops/mcp
go mod tidy
go build -o "$HOME/.local/bin/balkazar-musicops-mcp" .
```

Configure a supported LOCAL MCP client with a stdio command and fixed environment paths (replace all absolute paths):

```json
{
  "mcpServers": {
    "balkazar-musicops": {
      "command": "/ABS/PATH/TO/balkazar-musicops-mcp",
      "env": {
        "MUSICOPS_REVIEW_BUNDLE": "/ABS/PATH/TO/review-bundle.json",
        "MUSICOPS_LEDGER": "/ABS/PATH/TO/ledger.json"
      }
    }
  }
}
```

**This is not yet installed in ChatGPT.** MCP client availability varies; it has no public HTTP listener and no third-party write actions. Avoid exposing a browser debugging port, profile directory or MCP tool with credentials on the open internet.

## Testing

```sh
cd tools/gigops
go test ./...
cd browser && node --test profile.test.mjs && node --check login-songkick.mjs
cd ../mcp && go mod tidy && go test ./...
```

GitHub's listing-preview workflow tests both Go components, the browser-profile safety checks and the existing Python exporter; CI does not attempt Songkick login or a real browser.

## Next engineering milestones

1. Verify Balkazar is manageable in the existing Tourbox account and capture Songkick's **actual** artist ID and the correct venue record. Check an existing Dec 3 event before making a new one.
2. Obtain Songkick's permission for unattended browser publishing or a supported partner write integration. Keep the user-supervised Tourbox UI as the interim manual publication route.
3. With permission, implement restricted platform-specific Playwright actions: managed-account preflight → check drafts/live → validate identity/venue/time → dry-run → explicit approval → one publish → verify returned public event URL → update ledger/ClickUp and website backlink. Fail closed when selectors, MFA or terms change.
4. Only after a working local pilot, decide whether an existing private AWS host adds value; no new always-on infrastructure or cost for the current pilot.

Microsoft's official Playwright MCP can also attach to an existing browser or persistent profile, but never run two workers simultaneously against the same profile. Generic desktop-control or AI screen-coordinate automation is not needed for this stage.

## Google sign-in blocked by Playwright Chromium (2026-10-24)

The owner observed Google's **“Couldn't sign you in / This browser or app may not be secure”** within the previously Playwright-controlled Chromium browser. This is an OAuth browser-integrity check, not something to bypass. The launcher now uses a regular installed Chrome/Brave process and a **separate native profile**, without automation, remote debugging or cookie exports. From the phone-accessible Chrome Remote Desktop Xfce session, stop the previous terminal/browser session and use the Brave desktop shortcut immediately, or update the checked-out PR branch and run `npm run login:songkick` after fetching the new commit. If Google still refuses login, use Songkick's available supported direct credentials flow or contact support—do not add stealth flags, custom user agents or proxy tricks. Do not treat a login on a regular Chrome/Brave profile as an authenticated Playwright automation session.


## Media intake — read-only deterministic preflight (Nov 9, 2026)

The existing [MusicOps Media Assets tab](https://docs.google.com/spreadsheets/d/1BzP3pOfa2Zl2QPQMoOT2k8PSDklvZnopG3a-jSSraIE/edit#gid=115456632) is the **private catalog of record**. New Balkazar media goes into the existing [Drive Source Drop](https://drive.google.com/drive/folders/1c7PN6H0wdfJxBuKIvo1Z1Jj_637vvuTM) and existing Photos/Videos event folders. No raw media or private Sheets exports belong in the public GitHub repository.

The `media-plan` subcommand is a **no-network, no-write gate**. Feed it two PRIVATE, LOCAL connector snapshots: (1) the authenticated Drive `list_folder` response for the specific Balkazar Source Drop, saved as JSON in a private directory, and (2) CSV containing the existing Media Assets tab, exported locally with its exact header row. It matches files by actual Drive ID and rejects duplicate/corrupt source inventories. Identical filenames are *only* comparison candidates; no automatic deletes or byte-identity claims. Every genuinely new photo/video stays blocked for rights/date/participant review.

```sh
cd tools/gigops
go test ./...
go run ./cmd/musicops media-plan \
  --inventory "$HOME/.config/balkazar-musicops/private-source-drop.json" \
  --catalog "$HOME/.config/balkazar-musicops/private-media-assets.csv"
```

This is an engineering integrity check, **not a complete automated Drive watch or uploader**. The already-scheduled Balkazar Growth Operator (existing Mon/Wed/Fri 19:00 Asia/Taipei) remains responsible for authenticated Drive/Sheets reads and any verified, owner-authorized *private* row writes. It can run `media-plan` locally when its host environment supports it, or apply the same ID-based logic directly in the connectors. It should not run a parallel scraper or copy the private catalog into GitHub Actions.

Acceptance test: place a genuinely new permitted test asset in the source drop, verify exactly one new catalog entry after the agent's next successful run, verify a second run is a no-op, and verify zero publications/costs. GitHub CI executes only synthetic Go tests; GitHub CI does **not** have Drive credentials. Approval gates remain for public site merges, Metricool scheduling and publication, LINE/email, third-party event submissions, and artist/photographer rights. Never confuse a produced draft, a suggested time, or a linked URL with a live post.

**December 3 listing state correction:** Songkick was already published with public concert URL `https://www.songkick.com/concerts/43502730-balkazar-at-xiangti-avenue-plaza` and linked from both live website languages via merged PR #56 on Nov 6. Bandsintown was already published too. **Do not create a second Songkick or Bandsintown event.** Because the local Go ledger is private/on-device, it may still show `unknown` until synced/explicitly recorded; this is not evidence that a listing is missing. Before attempting any external submission, ALWAYS check live public backlinks and the private ClickUp campaign first. No unattended Tourbox publisher is authorized.



## Conversational gig intake → read-only plan (Nov 14, 2026)

**New first autonomous capability (draft PR only):** The AI/client may take Marko's natural-language gig announcement, extract **supported facts** into a `GigIntake` JSON object, call `musicops_preview_gig_intake` (local stdio MCP), and receive a deterministic list of blockers, selected candidate platforms, and a dependency-ordered campaign plan. The same code is usable in the local Go CLI:

```sh
cd tools/gigops
go run ./cmd/gig-intake --file examples/dec3-gig.json
# Or pipe exactly one structured, validated gig JSON via stdin.
go test ./...
cd mcp && go test ./...
```

The example is the confirmed Dec 3 **Balkazar guest appearance** (not a new organizer or ticketed event). The preview deliberately reports `"read_only": true` and `"public_writes": 0`; public event listings and social scheduling are candidates that still require a validated adapter and owner-scoped grant. A source URL supplied by chat is evidence to verify, **not** proof of live publication or ownership. The parser that converts natural language to structured gig data is the conversational assistant, **not** a claim that the Go CLI can understand speech itself.

Keep a **persistent** `gig_id` when changing an event date or venue. For a brand-new gig, the planner refuses to mark the booking ready until the existing registry has been checked and an ID is assigned; this protects against duplicates and reschedules. For tentative/held gigs, public publication stays blocked; cancellation produces a retract/reconcile workflow instead of generating promotion. Guest appearances exclude independent KKTIX/ACCUPASS organizer pages. Only Balkazar is currently configured; extending to Fuzzy Wall/solo requires separate tenant sources and permissions.

The plan is **not a scheduler, a website publisher, or a persistent execution database yet**. Next implementation increment: a durable per-gig job/authorization/receipt store that survives retries, checks authoritative links and integrates one existing supported publisher (Metricool MCP) under a revocable owner-defined policy. Do not create a paid service or bypass any platform's terms. For the broader acceptance criteria and progressive architecture see [CAMPAIGN_AUTOMATION.md](./CAMPAIGN_AUTOMATION.md).


## Persist and resume one gig's private campaign (Nov 14)

The **read-only** `musicops_preview_gig_intake` MCP tool and default CLI preview do **not** change anything. An agent running on an *authorized local machine* can optionally persist the validated plan into a separate, user-private local JSON campaign store via the same Go CLI:

```sh
cd tools/gigops
mkdir -p "$HOME/.config/balkazar-musicops"
go run ./cmd/gig-intake --file examples/dec3-gig.json \
  --store "$HOME/.config/balkazar-musicops/campaigns.json"
# Repeat the same command: "change": "unchanged", no duplicate campaign.
# Change the confirmed date for the SAME gig_id: revision increments,
# previously live or scheduled external destinations become review-required.
```

**Safety properties:** existing `campaigns.json` is loaded, validated and locked against concurrent writes; modifications use atomic replace and mode 0600. The .lock file stays private; do not delete it during concurrent use. The store rejects malformed/oversized files, duplicate canonical gig URLs or exact same artist/venue/start using another ID, and any path inside the public repository. Cancellation retains an approval-gated withdrawal plan and cannot schedule a new promotion. There are tests for idempotent repeats, reschedules, cancellation, missing ID, duplicate gigs, fail-closed corruption, concurrent locks, and unsafe file locations.

**Limitations:** this stores **internal planned jobs**, not autonomous execution status from third-party providers. The verified Songkick/Bandsintown URL ledger remains separate. This code never creates public events, posts, buys ads, sends DMs, loads private Google Drive media, or starts a live scheduler. ChatGPT does not yet expose a write-capable GigOps MCP connected to the user's local campaign store. Next capability is a trusted adapter that can read the job store, prepare one approved Metricool draft, and write back its verified provider receipt; no blanket publisher or user-secret access until explicit scoped authorization.
