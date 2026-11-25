"""Offline validation of official Songkick Tourbox import; no account use/publication."""

import csv
import tempfile
from pathlib import Path
import unittest

from export import ListingError, load_config
from songkick import (
    SONGKICK_HEADERS, ACKNOWLEDGEMENTS, blockers, build_songkick_fields,
    ensure_release_gate, export_songkick, read_official_template,
)

ROOT = Path(__file__).parent
ACK = {key: True for key in ACKNOWLEDGEMENTS}
SETTINGS = {
    "songkick_artist_id": "12345678",
    "songkick_artist_url": "https://www.songkick.com/artists/12345678-balkazar",
    "songkick_venue_name": "Xiangti Avenue Plaza",
    "songkick_venue_url": "https://www.songkick.com/venues/12345-xiangti-avenue-plaza",
    "songkick_public_event_url": None,
}


class SongkickTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.base, cls.event = load_config(ROOT / "2026-12-03.json")

    def test_official_public_template_19_columns(self):
        self.assertEqual(len(SONGKICK_HEADERS), 19)
        self.assertEqual(read_official_template(ROOT / "songkick-official-template.csv"),
                         SONGKICK_HEADERS)

    def test_bilingual_preview_and_source_times(self):
        values = build_songkick_fields(self.base, self.event, SETTINGS)
        self.assertEqual(values["Date*"], "2026-12-03")
        self.assertEqual(values["Time*"], "20:00")
        self.assertEqual(values["City*"], "Taipei")
        self.assertEqual(values["Country or State*"], "Taiwan")
        self.assertEqual(values["Event Name"], "Balkazar Live at the European Christmas Market")
        self.assertIn("Balkazar", values["Description"])
        self.assertIn("並非", values["Description"])
        self.assertEqual(values["Ticket Link"], "")
        self.assertEqual(values["Publish Date"], "")
        self.assertEqual(values["Publish Time"], "")

    def test_fail_closed_until_artist_and_venue_ids_known(self):
        self.assertTrue(blockers({}))
        missing = dict(SETTINGS, songkick_artist_id=None, songkick_venue_name=None)
        with self.assertRaises(ListingError):
            ensure_release_gate(missing, ACK)
        self.assertTrue(any("artist" in x for x in blockers(missing)))
        self.assertTrue(any("venue" in x for x in blockers(missing)))

    def test_fail_closed_if_duplicate_exists_or_approval_missing(self):
        with self.assertRaises(ListingError):
            ensure_release_gate({**SETTINGS, "songkick_public_event_url": "https://www.songkick.com/concerts/99-example"}, ACK)
        for key in ACKNOWLEDGEMENTS:
            with self.subTest(key=key):
                with self.assertRaises(ListingError):
                    ensure_release_gate(SETTINGS, {**ACK, key: False})

    def test_reject_fake_artist_url_or_venue_url(self):
        self.assertTrue(blockers({**SETTINGS, "songkick_artist_url": "https://www.songkick.com/artists/99999-wrong"}))
        self.assertTrue(blockers({**SETTINGS, "songkick_venue_url": "https://example.com/venues/1"}))

    def test_export_official_csv_filename_and_no_tickets(self):
        with tempfile.TemporaryDirectory() as folder:
            directory = Path(folder)
            target = export_songkick(
                self.base, self.event, SETTINGS, ROOT / "songkick-official-template.csv",
                directory, ACK,
            )
            self.assertEqual(target.name, "songkick-12345678-balkazar-2026-12-03.csv")
            with target.open(encoding="utf-8-sig", newline="") as stream:
                rows = list(csv.DictReader(stream))
            self.assertEqual(len(rows), 1)
            self.assertEqual(list(rows[0].keys()), list(SONGKICK_HEADERS))
            self.assertEqual(rows[0]["Venue*"], "Xiangti Avenue Plaza")
            self.assertEqual(rows[0]["Ticket Link"], "")
            self.assertEqual(rows[0]["Publish Date"], "")
            with self.assertRaises(ListingError):
                export_songkick(self.base, self.event, SETTINGS, ROOT / "songkick-official-template.csv", directory, ACK)

    def test_reject_template_drift_without_writing(self):
        with tempfile.TemporaryDirectory() as folder:
            directory = Path(folder)
            changed = directory / "changed.csv"
            changed.write_text("Venue*,New Mandatory Field*\n", encoding="utf-8")
            with self.assertRaises(ListingError):
                export_songkick(self.base, self.event, SETTINGS, changed, directory / "out", ACK)
            self.assertFalse((directory / "out").exists())

    def test_real_config_is_not_publish_ready(self):
        import json
        settings = json.loads((ROOT / "2026-12-03.songkick.json").read_text(encoding="utf-8"))
        self.assertTrue(blockers(settings))
        with self.assertRaises(ListingError):
            ensure_release_gate(settings, ACK)

if __name__ == "__main__":
    unittest.main()
