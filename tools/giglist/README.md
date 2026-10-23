# Balkazar event listings — review-first export (pilot)

Tracks GitHub issue #41: https://github.com/marko-durasic/balkazar.github.io/issues/41
Canonical ClickUp campaign task: https://app.clickup.com/t/z91464juf6

**Status:** Generates a LOCAL CSV only. It does not log into the artist account, upload, publish, notify followers, create tickets, spend money, or provision AWS. Balkazar's existing Bandsintown page is https://www.bandsintown.com/a/15495651-balkazar (artist ID 15495651). **Never create a duplicate artist page.**

## Why CSV first

Bandsintown documents a read-only artist/event API and an official CSV upload flow (up to 25 rows). The current upload guide says Upload creates drafts and Publish is a separate step, but an older help page says bulk upload publishes shortly afterward. **Do not assume upload is private**: the owner must authorize any import, and the operator must confirm the actual interface behavior. Use the *current* template downloaded after login; do not guess columns or try undocumented/private write APIs.
Official instructions: https://artists.bandsintown.com/support/event-upload

## Prepare December 3

1. In https://artists.bandsintown.com/ choose the existing **Balkazar** artist; inspect BOTH live and draft events for December 3 so nothing gets duplicated.
2. Under Events → Upload, download the official current CSV template. If it is XLSX, save as CSV UTF-8 first.
3. From the repository root run the commands below, substituting your real downloaded template path.

    python3 tools/giglist/export.py --config tools/giglist/2026-12-03.json

    python3 tools/giglist/export.py --config tools/giglist/2026-12-03.json --template /path/to/template.csv --output /tmp/balkazar-bandsintown-2026-12-03.csv

4. Inspect the resulting CSV. Check that Bandsintown matches the venue correctly. Never state ticket/free-entry terms without a source or suggest that Balkazar organizes the whole market. Event images are omitted until rights/consent are verified. The approved 28-column template includes decorated headers (e.g. `Start Date* (yyyy-mm-dd)`); the exporter handles these and sets `Do Not Announce=Y` in the review file.
5. The artist owner approved the **contents** of the October 23 CSV, **not upload or publication**. Obtain explicit approval before an import. Only use a verified draft-only flow; do not click Publish/Schedule or notify followers without additional approval. Once a public event URL is confirmed, save it in ClickUp and link to it from the Balkazar event page.

## Engineering guardrails

- Start/end/timezone, performer, venue and canonical link derive from the website's own MusicEvent JSON-LD. Supplemental bilingual copy, Bandsintown artist ID, and reviewed venue metadata are versioned in the config.
- The CSV uses the downloaded template's exact column order and labels (October 2026: 28 columns); it normalizes decorated labels like `Start Time* (HH:MM)`. Unknown required fields fail closed, unknown optional fields remain blank. UTF-8 BOM supports Traditional Chinese in spreadsheet applications.
- Ticket links, ticket labels, lineups, publicity schedule and photos are intentionally left empty. No auto-publication or third-party API calls occur.
- No private credentials/API keys are checked into GitHub. User login, draft inspection, venue identity matching and final publication remain manual approval gates.
- New gigs can use a second reviewed config referencing that event's page. This pilot is Balkazar only: Fuzzy Wall and Marko Djurasic are separate music entities.
- Future phases: Songkick Tourbox adapter after its current CSV template is obtained; persistent dedupe ledger; optional authenticated Playwright **draft** entry; only then evaluate existing AWS/GitHub Actions capacity. No always-on EC2 or new AWS spending for this pilot.

Tests: python3 -m unittest discover -s tools/giglist -p 'test_*.py'
