"""Validate autonomous review-bundle discovery without any external writes."""

import unittest

from review_bundle import build_review_bundle, event_key
from export import load_config
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class BundleTests(unittest.TestCase):
    def test_december_3_present_and_no_publication(self):
        bundle = build_review_bundle()
        self.assertTrue(bundle["read_only"])
        self.assertEqual(bundle["external_publications"], 0)
        item = next(x for x in bundle["events"] if x.get("url", "").endswith("2026-12-03-european-christmas-market/"))
        self.assertEqual(item["status"], "review-required-not-published")
        self.assertEqual(item["start"], "2026-12-03T20:00:00+08:00")
        self.assertEqual(item["venue"], "Xiangti Avenue Plaza")
        self.assertEqual(item["bandsintown_fields"]["Artist Name"], "Balkazar")
        self.assertTrue(item["songkick_blockers"])
        self.assertFalse(item["songkick_artist_verified"])
        self.assertFalse(item["songkick_venue_verified"])
        self.assertNotIn("published_event_url", item)

    def test_fingerprint_stable_for_same_event(self):
        _, event = load_config(ROOT / "tools/giglist/2026-12-03.json")
        self.assertEqual(event_key(event), event_key(dict(event)))
        self.assertNotEqual(event_key(event), event_key({**event, "startDate": "2026-12-04T20:00:00+08:00"}))


if __name__ == "__main__":
    unittest.main()
