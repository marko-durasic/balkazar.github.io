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

## Songkick / Tourbox (October 24, 2026)

**Artist access not yet verified:** Check the owner's existing Songkick/Tourbox account before registering another login or creating a Balkazar artist page. A previously managed artist does not establish that Balkazar is managed. Search Tourbox for Balkazar, then follow its official artist-management verification flow.

**Official import format:** Songkick's own downloadable 19-column template is committed as `songkick-official-template.csv`, from https://support.songkick.com/hc/en-us/article_attachments/360004841917 (verified 2026-10-24). Bulk instructions: https://support.songkick.com/hc/en-us/articles/360012427214-Adding-your-artist-tour-to-Songkick-with-the-Bulk-Upload-Tool . Direct event entry and approval: https://support.songkick.com/hc/en-us/articles/360012785193-How-to-add-events-to-Songkick-with-Tourbox-for-artists .

**One-time pilot (local preview is READY now):**

    python3 tools/giglist/songkick.py

The preview uses the same authoritative MusicEvent JSON-LD and bilingual wording as Bandsintown. Publication-blocking fields are deliberately `null` in `2026-12-03.songkick.json` until verified through Tourbox: Songkick artist ID + URL, **exact** Songkick database venue name + URL, and whether this event already exists. Search-engine non-results are not proof of absence; the event may be aggregated from another source.

**To reach an upload-ready file:** use the existing Tourbox login to add Balkazar to managed artists (artist verification may be requested), match/create the correct Xiangti Avenue Plaza Songkick venue, and check for a Dec 3 existing event. Record validated IDs/URLs in the per-platform settings JSON. Use a FRESH Tourbox-downloaded CSV and the exporter. It refuses to write unless every relevant command-line confirmation is explicitly supplied:

    python3 tools/giglist/songkick.py --template /path/to/current-tourbox-template.csv --output-dir /tmp/balkazar-songkick-review --confirmed-artist-access --confirmed-venue-match --confirmed-no-duplicate --acknowledge-import-may-publish

**CRITICAL:** Songkick's upload page may publish an event immediately if `Publish Date/Time/Timezone` are empty. Unlike Bandsintown, the Tourbox bulk import is NOT a safe draft-only action; do not upload the generated CSV until the actual UI and the title/date/venue are checked and the bandleader has authorized that public event. Approved ticket vendors only; leave ALL ticket-related columns blank since we cannot invent market tickets or imply Balkazar runs the entire market. The script never visits Songkick or presses Upload/Publish.

**For future gigs:** reuse source event JSON-LD + approved text and make one config per event/artist. Before each submission check Tourbox management access, existing calendar/artist events, exact venue match, and explicit user permission; track returned public event URL in ClickUp. Real authenticated publishing automation is still a separate milestone in GitHub issue #41. Build it through a dedicated consented signed-in browser/Work profile and verify live page/duplicate handling. Do NOT automate by assuming a public write API, copying browser cookies into GitHub, defeating MFA/CAPTCHA, or running an always-on paid EC2 instance.

**No website/marketing deployment is part of this PR.** Existing December 3 Bandsintown is already published, and must not be reuploaded. Tests include the Songkick official template, preview, release gates, duplicate protection and CSV hygiene.