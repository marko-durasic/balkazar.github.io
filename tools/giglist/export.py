#!/usr/bin/env python3
"""Review-first Bandsintown CSV export from a published Balkazar MusicEvent JSON-LD.

Only prepares local files. Does not authenticate, publish, call external APIs, or
create fake tickets/RSVPs. Use the *current* template downloaded by the artist.
"""

from __future__ import annotations

import argparse
import csv
import json
import re
from datetime import datetime
from html.parser import HTMLParser
from pathlib import Path
from zoneinfo import ZoneInfo

REPO_ROOT = Path(__file__).resolve().parents[2]
REQUIRED_HEADERS = {"Venue", "Country", "City", "Region", "Start Date", "Start Time", "Timezone"}
SUPPORTED_HEADERS = {
    "Venue", "Country", "Address", "City", "Region", "Postal Code", "Start Date",
    "End Date", "Start Time", "End Time", "Streaming Link", "Ticket Link",
    "Ticket Type", "Ticket Link 2", "Ticket Type 2", "On-Sale Date",
    "On-Sale Time", "Lineup", "Event Name", "Event Display Format",
    "Description", "Schedule Date", "Schedule Time", "Scheduled date",
    "Scheduled time", "Do Not Announce", "Setlist", "Timezone",
    "Artist Name", "Event Image",
}


class ListingError(ValueError):
    """Input or template cannot safely be exported."""


class JsonLdScripts(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=False)
        self._capturing = False
        self._parts: list[str] = []
        self.scripts: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag.lower() == "script" and (dict(attrs).get("type") or "").lower() == "application/ld+json":
            self._capturing = True
            self._parts = []

    def handle_data(self, data: str) -> None:
        if self._capturing:
            self._parts.append(data)

    def handle_endtag(self, tag: str) -> None:
        if tag.lower() == "script" and self._capturing:
            self.scripts.append("".join(self._parts))
            self._capturing = False


def music_events(value: object):
    if isinstance(value, list):
        for child in value:
            yield from music_events(child)
    elif isinstance(value, dict):
        types = value.get("@type", [])
        if isinstance(types, str):
            types = [types]
        if "MusicEvent" in types:
            yield value
        if "@graph" in value:
            yield from music_events(value["@graph"])


def get_event(html: str, url: str) -> dict:
    parser = JsonLdScripts()
    parser.feed(html)
    matches = []
    for raw in parser.scripts:
        try:
            matches.extend(ev for ev in music_events(json.loads(raw)) if ev.get("url") == url)
        except json.JSONDecodeError as exc:
            raise ListingError(f"Invalid JSON-LD: {exc}") from exc
    if len(matches) != 1:
        raise ListingError(f"Expected exactly one MusicEvent JSON-LD matching {url}, got {len(matches)}")
    return matches[0]


def local_datetimes(event: dict, timezone: str) -> tuple[datetime, datetime]:
    try:
        tz = ZoneInfo(timezone)
        start_raw = datetime.fromisoformat(event["startDate"])
        end_raw = datetime.fromisoformat(event["endDate"])
    except (KeyError, TypeError, ValueError) as exc:
        raise ListingError(f"Invalid start/end/timezone: {exc}") from exc
    if start_raw.tzinfo is None or end_raw.tzinfo is None:
        raise ListingError("Event date-times must include a UTC offset")
    start, end = start_raw.astimezone(tz), end_raw.astimezone(tz)
    if start.utcoffset() != start_raw.utcoffset() or end.utcoffset() != end_raw.utcoffset():
        raise ListingError("Event's UTC offset does not match the configured timezone")
    if end <= start or end.date() != start.date():
        raise ListingError("For this single-day gig, end must be after start on the same local day")
    return start, end


def bandsintown_values(config: dict, event: dict) -> dict[str, str]:
    artist = config["artist"]
    performer = event.get("performer")
    if not isinstance(performer, dict) or performer.get("name") != artist:
        raise ListingError("Source event performer does not match the configured artist")
    if event.get("eventStatus") != "https://schema.org/EventScheduled":
        raise ListingError("Event must be officially marked scheduled in its source JSON-LD")
    loc = event.get("location", {})
    address = loc.get("address", {})
    if not loc.get("name") or address.get("addressCountry") != config["country_code"]:
        raise ListingError("Venue name or country differs from the reviewed location")
    if address.get("addressRegion") != config["region"]:
        raise ListingError("Venue region differs from the reviewed location")
    if not config.get("artist_id") or not config.get("listing_title_en"):
        raise ListingError("Artist ID and clear performance-specific listing title are required")
    start, end = local_datetimes(event, config["timezone"])
    if event.get("url") != config["event_page_url"]:
        raise ListingError("Event URL differs from the reviewed source page")
    description_en = config["description_en"].strip()
    description_zh = config["description_zh"].strip()
    if not description_en or not description_zh:
        raise ListingError("Both English and Traditional Chinese descriptions must be reviewed")
    description = f"{description_en} {description_zh} Details / 演出資訊: {event['url']}"
    return {
        "Venue": loc["name"],
        "Country": config["country"], "Address": address.get("streetAddress", ""),
        "City": config["city"], "Region": config["region"],
        "Postal Code": address.get("postalCode", ""),
        "Start Date": start.strftime("%Y-%m-%d"),
        "Start Time": start.strftime("%H:%M"),
        "End Date": end.strftime("%Y-%m-%d"), "End Time": end.strftime("%H:%M"),
        "Timezone": config["timezone"], "Artist Name": artist,
        "Event Name": config["listing_title_en"], "Description": description,
        "Do Not Announce": "Y",
        # Intentionally absent: all ticket/RSVP/sales fields, event photo, and auto-publish schedule.
        # A free market appearance is NOT a sale/ticket inventory or organizer-owned event.
    }


def normalized_header(header: str) -> str:
    """Normalize actual Bandsintown headers, e.g. Start Date* (yyyy-mm-dd)."""
    header = header.lstrip("\ufeff").strip()
    header = re.sub(r"\s*\([^)]*\)\s*$", "", header)
    return header.replace("*", "").strip()


def export_csv(template: Path, output: Path, values: dict[str, str]) -> None:
    with template.open("r", encoding="utf-8-sig", newline="") as source:
        reader = csv.reader(source)
        headers = next(reader, None)
    if not headers:
        raise ListingError("Downloaded Bandsintown CSV template has no header row")
    normalized = [normalized_header(head) for head in headers]
    if len(normalized) != len(set(normalized)):
        raise ListingError("Duplicate field names in current upload template")
    missing = REQUIRED_HEADERS - set(normalized)
    unexpected_required = {normalized_header(h) for h in headers if "*" in h} - SUPPORTED_HEADERS
    if missing or unexpected_required:
        raise ListingError(f"Template changed; missing required {sorted(missing)}; unsupported required {sorted(unexpected_required)}")
    # Template remains authoritative. Preserve headers/order and leave all unknown optional fields blank.
    row = [values.get(field, "") for field in normalized]
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", encoding="utf-8-sig", newline="") as target:
        writer = csv.writer(target)
        writer.writerow(headers)
        writer.writerow(row)


def load_config(path: Path) -> tuple[dict, dict]:
    config = json.loads(path.read_text(encoding="utf-8"))
    source = (REPO_ROOT / config["source_html"]).resolve()
    if not source.is_relative_to(REPO_ROOT):
        raise ListingError("Source HTML must be within the Balkazar repository")
    event = get_event(source.read_text(encoding="utf-8"), config["event_page_url"])
    return config, event


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True, help="Reviewed per-gig JSON configuration")
    parser.add_argument("--template", type=Path, help="CSV downloaded fresh from Bandsintown for Artists > Events > Upload")
    parser.add_argument("--output", type=Path, help="Path to generated local CSV; file is not uploaded")
    args = parser.parse_args()
    try:
        config, event = load_config(args.config)
        values = bandsintown_values(config, event)
        if bool(args.template) != bool(args.output):
            raise ListingError("Pass both --template and --output, or neither for read-only preview")
        if args.template:
            export_csv(args.template, args.output, values)
            print(f"Saved LOCAL draft-import CSV to {args.output}. NOT uploaded or published.")
        else:
            print(json.dumps({"artist_id": config["artist_id"], "review_only": True,
                              "no_external_publication": True, "bandsintown_fields": values},
                             ensure_ascii=False, indent=2))
        return 0
    except (ListingError, OSError, KeyError, json.JSONDecodeError) as exc:
        parser.exit(2, f"Cannot safely export: {exc}\n")


if __name__ == "__main__":
    raise SystemExit(main())
