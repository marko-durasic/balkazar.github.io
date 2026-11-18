---
name: music-first-live-promo
description: Use when editing promotional videos for a live band's upcoming gig, especially if still-photo inserts feel random or flash too briefly, or if musical jump cuts make the performance feel unnatural.
---

# Music-first live promo editing

**Principle:** The performance is the film. Lock an authentic continuous musical passage first. Every photo adds a purposeful visual idea, not just a change.

## Approved creative reference — 2026-11-14
- Balkazar V8 is the **bandleader-approved creative quality reference**. This approval does **not** authorize publication.
- Read sibling [QUALITY_STANDARD.md](QUALITY_STANDARD.md) before building a new show promo; use [v8_approved_reference.json](v8_approved_reference.json) for the exact approved file SHA-256, provenance, audio source window and visual-protection/cutaway timing.
- Use `python -m unittest discover -s . -p 'test_*.py'` for the five known failure-mode regressions and `python verify_music_first_reference.py --reference v8_approved_reference.json` for metadata validation. With the exact video/source media available, pass `--video` and `--source` to verify the approved MP4 and audio against originals. These gates check integrity, **not** aesthetic equivalence.
- Maintain a new change log/test for meaningful editor feedback; do not overwrite the golden reference with future experiments.

## Inputs / trust
- Use an actual confirmed gig record and rights-cleared source audio, photos, and footage. Record provenance/event year. Do not imply old images show the current lineup or that different years are simultaneous camera angles.
- NT$0, local/private by default. Preserve originals and previous cuts. Require Marko's explicit approval of exact video, copy, channels, and timing before publishing. Do not contact others automatically.

## Workflow
1. **Lock music:** Choose one coherent original recording phrase, with a memorable hook and payoff. Prefer one unbroken live audio segment; never force a fast-to-slow musical jump. Listen to every transition.
2. **Earn the hook:** Start with genuine dancing and sound that rewards the viewer immediately. Overlay only clear band identity; do not block faces or instruments.
3. **Storyboard / pokrivalice:** Treat photos and extra camera views as picture-only cutaways over the continuous main performance audio. Protect visually meaningful moments: if the lead singer starts singing and the source shows them, let viewers SEE the singer instead of hiding that entrance beneath an unrelated instrument photo. Place horn, drum, band and audience inserts between protected lead moments, with an explicit story role; never insert a random musician just to create a cut.
4. **Beat and phrase timing:** Use measured beats/onsets as candidate markers (FFmpeg + librosa). Validate by listening; ska, cumbia and live tempo changes fool generic beat detectors. Cut on phrase changes and selected strong pulses; never force a cut on every beat.
5. **Photo pacing:** No universal hold length. Choose based on musical phrase, subject complexity and lead-performance visibility. On Balkazar's 18-second reel, 0.7-second flashing photos were too rushed AND 2.3-second solo-musician photos were too intrusive; several distinct, purposeful 1.0–1.2-second cutaways can work better. Keep small restrained 1–3% motion, intentional crops and coherent color; compare the actual edit rather than enforcing fixed seconds.
6. **Coherence:** Keep moving footage as the visual backbone. Photos are B-roll on the picture track only; underlying live audio stays intact. Use restrained, consistent transitions (brief ~0.1s visual blend is fine).
7. **CTA:** Put accurate bilingual performance date/time/location/free entry in a phone-safe area. Do not force viewers to read text during a photo moment.
8. **Preflight:** Export an MP4, editable source/manifest, and contact sheet. Check first 3 seconds, every photo entrance/exit, last 3 seconds, and platform safe zones. ffprobe validates H.264/AAC and 9:16. Upscaling doesn't create missing original detail.

## Decision test
- For each photo: preserve an explicit `story_role`, `start`, `end`, `motion`, ownership/rights and source-year notes.
- Check holds against measured beat candidates and require meaningful readability. Mechanical timing checks are not a guarantee of professional editing; final ear/eye review by a musician wins.
- Compare to prior candidate on musical continuity, visual purpose, readability and truthful audience invitation; not just cut count.
- Private review only until release approval; no made-up crowd size, false urgency or fake performance content.

## Project test
Balkazar V5 had three visually abrupt photos at 0.72, 1.14 and 0.74 seconds. V7 slowed the photos to ~2.3 seconds, but one trombone shot overlapped the on-camera start of the singer's vocal from timeline 5.55–7.81. That was a real editorial failure despite passing simple timing heuristics. V8 keeps one unbroken 18.26-second live audio section, protects the on-camera singer from 4.38–8.557, and places four ~1.1-second, story-tagged photos outside it. These timestamps are specific to this edit, NOT universal rules. Test the protected lead interval and evaluate entrances/exits by ear and eye.

## Reference techniques (do not blindly install tool code)
- https://github.com/vidmoat/skills (music-montage, beat-sync, pacing; CC-BY-4.0 text)
- https://github.com/remotion-dev/skills (official Remotion production guides)
- https://github.com/WeftCut/WeftCut (optional reversible MCP timeline pilot)
