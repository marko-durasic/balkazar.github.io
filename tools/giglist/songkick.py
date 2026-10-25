#!/usr/bin/env python3
"""Prepare Songkick Tourbox's official CSV safely from Balkazar's published MusicEvent.

Preview is always read-only. Export is a local file operation, NOT an upload:
a Songkick bulk import may publish immediately. Never upload it unattended.
"""

from __future__ import annotations

import argparse
import csv
import json
import re
from pathlib import Path

from export import REPO_ROOT, ListingError, bandsintown_values, load_config, local_datetimes

# The 19 fields in Songkick's *public official* bulk-upload template, checked 2026-10-24:
# https://support.songkick.com/hc/en-us/article_attachments/360004841917
SONGKICK_HEADERS = (
    "Venue*", "City*", "Country or State*", "Address", "Date*", "Time*",
    "Duration", "Ticket Link", "Ticket Type", "Ticket Link 2",
    "Ticket Type 2", "On-Sale Date", "On-Sale Time", "Lineup",
    "Event Name", "Description", "Publish Date", "Publish Time", "Publish Timezone",
)

ACKNOWLEDGEMENTS = (
    "confirmed_artist_access", "confirmed_venue_match", "confirmed_no_duplicate",
    "acknowledge_import_may_publish",
)


def build_songkick_fields(base: dict, event: dict, settings: dict) -> dict[str, str]:
    # Reuse existing quality gates: performer, source page, time zone, country,
    # region, and bilingual approved copy must match authoritative JSON-LD.
    bandsintown_values(base, event)
    start, _ = local_datetimes(event, base["timezone"])
    place = event["location"]
    source_name = place["name"]
    venue = settings.get("songkick_venue_name") or ""
    description = (
        f"{base['description_en'].strip()}\n"
        f"{base['description_zh'].strip()}\n"
        f"Details / 演出資訊: {event['url']}"
    )
    return {
        # The exact SONGKICK database venue name is NOT established by the
        # website's human-readable Xiangti Avenue Plaza name. Export requires
        # confirmed Songkick venue URL and exact display name separately.
        "Venue*": venue,
        "City*": base["city"],
        "Country or State*": base["country"],
        "Address": place.get("address", {}).get("streetAddress", ""),
        "Date*": start.strftime("%Y-%m-%d"),
        "Time*": start.strftime("%H:%M"),
        # Leaving Duration blank is intentional: Songkick's public CSV docs
        # do not specify its unit/format; this prevents a misleading value.
        "Duration": "",
        "Ticket Link": "",
        "Ticket Type": "",
        "Ticket Link 2": "",
        "Ticket Type 2": "",
        "On-Sale Date": "",
        "On-Sale Time": "",
        # Tourbox knows the headliner from the artist account/filename.
        "Lineup": "",
        "Event Name": base["listing_title_en"],
        "Description": description,
        # Empty schedule fields may cause IMMEDIATE Songkick publication on
        # upload. This file MUST NOT be submitted without explicit approval.
        "Publish Date": "",
        "Publish Time": "",
        "Publish Timezone": "",
    }


def blockers(settings: dict) -> list[str]:
    missing = []
    artist_id = str(settings.get("songkick_artist_id") or "")
    artist_url = str(settings.get("songkick_artist_url") or "")
    venue_name = str(settings.get("songkick_venue_name") or "")
    venue_url = str(settings.get("songkick_venue_url") or "")

    if not (re.fullmatch(r"[1-9]\d*", artist_id) and
            re.fullmatch(r"https://(?:www\.)?songkick\.com/artists/" + re.escape(artist_id) + r"(?:-[^/\s?#]+)?/?", artist_url)):
        missing.append("Verified Songkick artist ID and matching public artist URL")
    if not (venue_name and re.fullmatch(
            r"https://(?:www\.)?songkick\.com/venues/[1-9]\d*(?:-[^/\s?#]+)?/?", venue_url)):
        missing.append("Exact Songkick database venue name and matching venue URL")
    if settings.get("songkick_public_event_url"):
        missing.append("A Songkick event URL is already recorded; edit existing listing instead of duplicate upload")
    return missing


def ensure_release_gate(settings: dict, acknowledgement: dict[str, bool]) -> str:
    problems = blockers(settings)
    problems.extend(k.replace("_", " ") for k in ACKNOWLEDGEMENTS if not acknowledgement.get(k, False))
    if problems:
        raise ListingError("Songkick import CSV blocked: " + "; ".join(problems))
    return str(settings["songkick_artist_id"])


def read_official_template(path: Path) -> tuple[str, ...]:
    with path.open(encoding="utf-8-sig", newline="") as stream:
        header = next(csv.reader(stream), None)
    if tuple(header or ()) != SONGKICK_HEADERS:
        raise ListingError("Tourbox CSV columns differ from official 19-column support template; review updated schema before export")
    return tuple(header)


def export_songkick(
    base: dict, event: dict, settings: dict, template: Path, output_dir: Path,
    acknowledgement: dict[str, bool],
) -> Path:
    artist_id = ensure_release_gate(settings, acknowledgement)
    headers = read_official_template(template)
    values = build_songkick_fields(base, event, settings)
    if not values["Venue*"]:
        raise ListingError("No verified Songkick venue selected")
    filename = f"songkick-{artist_id}-balkazar-{values['Date*']}.csv"
    output_dir = output_dir.resolve()
    # This is potentially publish-ready material, never commit it to a public repo.
    if output_dir.is_relative_to(REPO_ROOT):
        raise ListingError("Use a private output directory OUTSIDE the public website repository")
    output_dir.mkdir(parents=True, exist_ok=True)
    target = output_dir / filename
    if target.exists():
        raise ListingError(f"Refusing to overwrite existing Songkick CSV: {target}")
    with target.open("w", encoding="utf-8-sig", newline="") as output:
        writer = csv.writer(output)
        writer.writerow(headers)
        writer.writerow([values[name] for name in headers])
    return target


def main() -> int:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--base", type=Path, default=Path(__file__).parent / "2026-12-03.json")
    cli.add_argument("--settings", type=Path, default=Path(__file__).parent / "2026-12-03.songkick.json")
    cli.add_argument("--template", type=Path, help="Official CSV obtained from current Tourbox account, NOT a guessed schema")
    cli.add_argument("--output-dir", type=Path, help="Private output directory for a local CSV file")
    cli.add_argument("--confirmed-artist-access", action="store_true")
    cli.add_argument("--confirmed-venue-match", action="store_true")
    cli.add_argument("--confirmed-no-duplicate", action="store_true")
    cli.add_argument("--acknowledge-import-may-publish", action="store_true")
    options = cli.parse_args()
    try:
        base, event = load_config(options.base)
        settings = json.loads(options.settings.read_text(encoding="utf-8"))
        preview = build_songkick_fields(base, event, settings)
        if not options.template and not options.output_dir:
            print(json.dumps({
                "platform": "Songkick / Tourbox",
                "status": "read-only-preview",
                "artist": base["artist"],
                "blocking_requirements": blockers(settings),
                "public_event_published": False,
                "csv_fields": preview,
            }, indent=2, ensure_ascii=False))
            return 0
        if not options.template or not options.output_dir:
            raise ListingError("Pass BOTH --template and --output-dir for a local export")
        ack = {key: bool(getattr(options, key)) for key in ACKNOWLEDGEMENTS}
        target = export_songkick(base, event, settings, options.template, options.output_dir, ack)
        print(f"Local Songkick CSV generated: {target}. NOT uploaded or published. Treat import as publication-capable.")
        return 0
    except (ListingError, OSError, KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
        cli.exit(2, f"Cannot prepare Songkick submission: {exc}\n")


if __name__ == "__main__":
    raise SystemExit(main())
