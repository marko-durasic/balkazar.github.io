#!/usr/bin/env python3
"""Build safe per-gig listing previews from Balkazar public event pages.

Does NOT log in, upload, send notifications, create tickets, or schedule publishing.
Use on CI whenever a gig page/config changes; artifacts are for human review.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

from export import REPO_ROOT, ListingError, bandsintown_values, get_event, load_config
from songkick import build_songkick_fields, blockers


def event_key(event: dict) -> str:
    """Stable content fingerprint for comparing updates; not proof of publication."""
    data = {
        "artist": event.get("performer", {}).get("name"),
        "url": event.get("url"),
        "startDate": event.get("startDate"),
        "endDate": event.get("endDate"),
        "location": event.get("location"),
    }
    canonical = json.dumps(data, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def build_review_bundle(root: Path = REPO_ROOT) -> dict:
    configs = {}
    for path in sorted((root / "tools" / "giglist").glob("*.json")):
        if path.name.endswith(".songkick.json"):
            continue
        raw = json.loads(path.read_text(encoding="utf-8"))
        if not raw.get("source_html"):
            continue
        source = raw["source_html"]
        if source in configs:
            raise ListingError(f"Multiple gig configurations point to {source}")
        configs[source] = path

    events = []
    seen = set()
    for html in sorted((root / "events").glob("*/index.html")):
        page_url = f"https://balkazar.org/events/{html.parent.name}/"
        try:
            event = get_event(html.read_text(encoding="utf-8"), page_url)
        except ListingError:
            # Some archived/special website pages may not be MusicEvent pages.
            # Never fabricate concert data; identify pages requiring inspection.
            events.append({"source_page": str(html.relative_to(root)),
                           "status": "review-source-jsonld", "url": page_url})
            continue
        source_html = str(html.relative_to(root))
        if event["url"] in seen:
            raise ListingError(f"Duplicate canonical event URL: {event['url']}")
        seen.add(event["url"])
        item = {"source_page": source_html, "url": event["url"], "fingerprint": event_key(event),
                "start": event["startDate"], "end": event["endDate"],
                "venue": event.get("location", {}).get("name"),
                "status": "needs-approved-bilingual-config"}
        if source_html in configs:
            base, checked_event = load_config(configs[source_html])
            fields = bandsintown_values(base, checked_event)
            songkick_settings_path = configs[source_html].with_name(
                configs[source_html].stem + ".songkick.json"
            )
            settings = (
                json.loads(songkick_settings_path.read_text(encoding="utf-8"))
                if songkick_settings_path.exists() else {}
            )
            item["status"] = "review-required-not-published"
            item["bandsintown_fields"] = fields
            item["songkick_fields"] = build_songkick_fields(base, checked_event, settings)
            item["songkick_blockers"] = blockers(settings)
            item["songkick_artist_verified"] = not any("artist" in m.lower() for m in blockers(settings))
            item["songkick_venue_verified"] = not any("venue" in m.lower() for m in blockers(settings))
            item["release_recommendation"] = (
                "Check exact artist + venue records, live/draft duplicates, and explicit consent. "
                "CSV upload to Songkick may publish immediately. Never infer live status from this bundle."
            )
        events.append(item)
    unlinked = [str(path.relative_to(root)) for source, path in configs.items() if not (root / source).exists()]
    if unlinked:
        raise ListingError(f"Gig config references missing website pages: {unlinked}")
    return {
        "schema_version": 1,
        "read_only": True,
        "external_publications": 0,
        "events": events,
        "documentation": "https://github.com/marko-durasic/balkazar.github.io/issues/41",
    }


def main() -> int:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--out", type=Path, help="Write local JSON bundle for CI artifact")
    opts = cli.parse_args()
    try:
        data = build_review_bundle()
        output = json.dumps(data, indent=2, ensure_ascii=False, sort_keys=True) + "\n"
        if opts.out:
            opts.out.parent.mkdir(parents=True, exist_ok=True)
            opts.out.write_text(output, encoding="utf-8")
            print(f"Wrote READ-ONLY gig-listing review bundle: {opts.out}. Nothing published.")
        else:
            print(output)
        return 0
    except (ListingError, OSError, KeyError, TypeError, ValueError) as err:
        cli.exit(2, f"Review-bundle generation failed: {err}\n")


if __name__ == "__main__":
    raise SystemExit(main())
