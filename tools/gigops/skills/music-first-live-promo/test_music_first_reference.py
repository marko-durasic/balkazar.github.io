#!/usr/bin/env python3
"""Regression tests: avoid repeating proven V5/V7 editing failures."""
import copy
import json
import unittest
from pathlib import Path
from verify_music_first_reference import audit_reference

BASE = Path(__file__).with_name("v8_approved_reference.json")

class MusicFirstTests(unittest.TestCase):
    def setUp(self):
        self.ref = json.loads(BASE.read_text(encoding="utf-8"))

    def test_v8_reference_is_valid(self):
        self.assertEqual([], audit_reference(self.ref))

    def test_v7_trombone_hides_singer(self):
        bad = copy.deepcopy(self.ref)
        bad["cutaways"][1]["start"] = 5.55
        bad["cutaways"][1]["end"] = 7.81
        self.assertTrue(any("protected lead" in e for e in audit_reference(bad)))

    def test_v5_audio_jump_forbidden(self):
        bad = copy.deepcopy(self.ref)
        bad["main_source"]["audio_continuous"] = False
        self.assertIn("Reference music must be continuous", audit_reference(bad))

    def test_photo_must_have_story_role_and_year(self):
        bad = copy.deepcopy(self.ref)
        bad["cutaways"][0]["story_role"] = ""
        self.assertTrue(any("purpose/provenance" in e for e in audit_reference(bad)))

    def test_creative_approval_is_not_publish_authorization(self):
        bad = copy.deepcopy(self.ref)
        bad["approval"]["publication"] = "AUTHORIZED"
        self.assertTrue(any("publication" in e.lower() for e in audit_reference(bad)))

if __name__ == "__main__":
    unittest.main()
