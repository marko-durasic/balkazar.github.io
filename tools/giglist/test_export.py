"""No network, credentials, third-party publishing or extra dependencies needed."""

import csv
import json
from pathlib import Path
import tempfile
import unittest

from export import ListingError, bandsintown_values, export_csv, get_event, local_datetimes, REPO_ROOT, load_config


EVENT = {
    "@context": "https://schema.org", "@type": "MusicEvent",
    "name": "Balkazar at European Christmas Market",
    "url": "https://balkazar.org/events/2026-12-03-european-christmas-market/",
    "startDate": "2026-12-03T20:00:00+08:00",
    "endDate": "2026-12-03T21:30:00+08:00",
    "eventStatus": "https://schema.org/EventScheduled",
    "performer": {"@type": "MusicGroup", "name": "Balkazar"},
    "location": {"@type": "Place", "name": "Xiangti Avenue Plaza", "address": {
        "@type": "PostalAddress", "streetAddress": "Songzhi Rd", "addressRegion": "Taipei City",
        "addressCountry": "TW", "postalCode": "110"}},
}
CFG = {
    "artist": "Balkazar", "artist_id": "15495651", "timezone": "Asia/Taipei",
    "country_code": "TW", "country": "Taiwan", "city": "Taipei", "region": "Taipei City",
    "event_page_url": EVENT["url"], "listing_title_en": "Balkazar Live at the European Christmas Market",
    "description_en": "Balkazar's performance inside the market.",
    "description_zh": "Balkazar 在歐洲聖誕市集中的演出，並非市集主辦單位公告。",
}


class GiglistTests(unittest.TestCase):
    def test_extracts_unique_matching_jsonld(self):
        html = '<script type="application/ld+json">' + json.dumps({"@graph": [EVENT]}) + '</script>'
        self.assertEqual(get_event(html, EVENT["url"])["location"]["name"], "Xiangti Avenue Plaza")
        with self.assertRaises(ListingError):
            get_event(html, "https://wrong.test/")

    def test_values_no_fake_tickets_and_correct_time(self):
        values = bandsintown_values(CFG, EVENT)
        self.assertEqual(values["Start Date"], "2026-12-03")
        self.assertEqual(values["Start Time"], "20:00")
        self.assertEqual(values["End Time"], "21:30")
        self.assertEqual(values["Timezone"], "Asia/Taipei")
        self.assertEqual(values["Venue"], "Xiangti Avenue Plaza")
        self.assertNotIn("Ticket Link", values)
        self.assertNotIn("Event Image", values)
        self.assertIn("並非", values["Description"])

    def test_rejects_bad_time_and_wrong_artist(self):
        broken = {**EVENT, "startDate": "2026-12-03T20:00:00"}
        with self.assertRaises(ListingError):
            local_datetimes(broken, "Asia/Taipei")
        with self.assertRaises(ListingError):
            bandsintown_values({**CFG, "artist": "Another Band"}, EVENT)

    def test_template_is_authoritative_and_extra_optional_fields_blank(self):
        values = bandsintown_values(CFG, EVENT)
        headers = ["Venue*", "Country*", "City*", "Start Date*", "Start Time*", "Timezone*",
                   "Artist Name", "Event Name", "Description", "Ticket Link", "Ticket Type", "Unknown Optional"]
        with tempfile.TemporaryDirectory() as tmp:
            template, output = Path(tmp) / "template.csv", Path(tmp) / "exports" / "upload.csv"
            template.write_text(",".join(headers) + "\n", encoding="utf-8")
            export_csv(template, output, values)
            with output.open(encoding="utf-8-sig", newline="") as stream:
                rows = list(csv.DictReader(stream))
            self.assertEqual(len(rows), 1)
            self.assertEqual(rows[0]["Venue*"], "Xiangti Avenue Plaza")
            self.assertEqual(rows[0]["Ticket Link"], "")
            self.assertEqual(rows[0]["Unknown Optional"], "")

    def test_rejects_changed_required_template(self):
        with tempfile.TemporaryDirectory() as tmp:
            template, output = Path(tmp) / "template.csv", Path(tmp) / "out.csv"
            template.write_text("Venue*,City*,Unrecognized New Field*\n", encoding="utf-8")
            with self.assertRaises(ListingError):
                export_csv(template, output, bandsintown_values(CFG, EVENT))
            self.assertFalse(output.exists())

    def test_real_website_jsonld_remains_authoritative(self):
        # Enforced in PR CI against the real site source, so website changes cannot silently drift.
        cfg, event = load_config(Path(__file__).parent / "2026-12-03.json")
        values = bandsintown_values(cfg, event)
        self.assertEqual(values["Start Time"], "20:00")
        self.assertEqual(values["End Time"], "21:30")
        self.assertEqual(values["Venue"], "Xiangti Avenue Plaza")
        self.assertEqual(REPO_ROOT / cfg["source_html"], REPO_ROOT / "events/2026-12-03-european-christmas-market/index.html")


if __name__ == "__main__":
    unittest.main()
