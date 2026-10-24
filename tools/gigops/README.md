# MusicOps — local Songkick pilot (review-only)

[Engineering issue #41](https://github.com/marko-durasic/balkazar.github.io/issues/41) · [Event-listings ClickUp](https://app.clickup.com/t/z91464juf6) · [Draft PR #53](https://github.com/marko-durasic/balkazar.github.io/pull/53)

**NOT a live Songkick publisher.** These components cannot create, edit, scrape, or publish Songkick events. They are an authenticated-browser bootstrap and a read-only, self-hostable coordination foundation. No AWS VM, external server, paid tool, or credentials are required to test them.

## Existing pieces

- `tools/giglist/` already produces per-gig bilingual event metadata from the website's `MusicEvent` JSON-LD and CI read-only `giglist-review` artifacts.
- `tools/gigops/`: minimal dependency-free Go planner, source URL/fingerprint validator and **private local** idempotent listing ledger. It records public links only after you verify them, and NEVER submits to platforms.
- `tools/gigops/mcp/`: official Go MCP SDK-based **stdio-only** plugin with TWO read-only tools, `musicops_list_gig_plans` and `musicops_get_gig_plan`. It cannot access arbitrary paths through tool inputs or browse on its own. Local MCP client must support running the server.
- `tools/gigops/browser/`: Playwright boots a separate visible Chromium profile, opens Tourbox, then allows YOU to log in and check the artist; doesn't inspect page data or click/upload/publish. The persistent profile is stored outside the public repo.

## Start here — complete Linux laptop checkout and Songkick login

**The following commands include cloning GitHub and checking out draft PR #53.** They work for a fresh checkout and do not run any Songkick publishing actions. Run them in a normal graphical Linux desktop terminal (not AWS/SSH/CI).

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

A fresh visible browser opens `https://tourbox.songkick.com/`. You manually sign in and handle MFA. In `Your artists`, check whether **Balkazar** already exists; if absent, use official `Add artist` and Songkick's requested relationship verification. Do not assume access because the personal Songkick login exists. Verify the Songkick artist page and the actual venue record for Xiangti Avenue Plaza; inspect December 3 drafts/live listings before considering an event submission.

Press Enter in the terminal to close the browser and retain the isolated session profile. The default directory is the user's application-data directory (not this repo); to override it set `MUSICOPS_BROWSER_PROFILE` to an **absolute private path**. Profile cookies/passwords/MFA/session exports must never go in GitHub, ChatGPT, a public URL or a GitHub Action. Sessions can expire and must sometimes be refreshed manually. One browser can use a given profile at a time.

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
