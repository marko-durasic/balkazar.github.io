---
name: music-first-live-promo
description: Use when editing promotional videos for a live band's upcoming gig, especially if still-photo inserts feel random or flash too briefly, or if musical jump cuts make the performance feel unnatural.
---

# Music-first live promo editing

**Principle:** The performance is the film. Lock an authentic continuous musical passage first. Every photo adds a purposeful visual idea, not just a change.

## Inputs / trust
- Use an actual confirmed gig record and rights-cleared source audio, photos, and footage. Record provenance/event year. Do not imply old images show the current lineup or that different years are simultaneous camera angles.
- NT$0, local/private by default. Preserve originals and previous cuts. Require Marko's explicit approval of exact video, copy, channels, and timing before publishing. Do not contact others automatically.

## Workflow
1. **Lock music:** Choose one coherent original recording phrase, with a memorable hook and payoff. Prefer one unbroken live audio segment; never force a fast-to-slow musical jump. Listen to every transition.
2. **Earn the hook:** Start with genuine dancing and sound that rewards the viewer immediately. Overlay only clear band identity; do not block faces or instruments.
3. **Storyboard:** Give each photo an intentional story role: horn shot while horns are heard, crowd selfie when audience excitement is audible, and return to moving band footage for payoff. Avoid random b-roll just to fill beats.
4. **Beat and phrase timing:** Use measured beats/onsets as candidate markers (FFmpeg + librosa). Validate by listening; ska, cumbia and live tempo changes fool generic beat detectors. Cut on phrase changes and selected strong pulses; never force a cut on every beat.
5. **Photo legibility:** For an ~18-second performance clip, first try 1.8–2.5-second holds for each still instead of subsecond flashes, unless a shorter cut is explicitly meaningful. Use restrained ~2–4% slow pan/zoom, purposeful cropping and consistent color; don't make random quick zooms.
6. **Coherence:** Keep moving footage as the visual backbone. Photos are B-roll on the picture track only; underlying live audio stays intact. Use restrained, consistent transitions (brief ~0.1s visual blend is fine).
7. **CTA:** Put accurate bilingual performance date/time/location/free entry in a phone-safe area. Do not force viewers to read text during a photo moment.
8. **Preflight:** Export an MP4, editable source/manifest, and contact sheet. Check first 3 seconds, every photo entrance/exit, last 3 seconds, and platform safe zones. ffprobe validates H.264/AAC and 9:16. Upscaling doesn't create missing original detail.

## Decision test
- For each photo: preserve an explicit `story_role`, `start`, `end`, `motion`, ownership/rights and source-year notes.
- Check holds against measured beat candidates and require meaningful readability. Mechanical timing checks are not a guarantee of professional editing; final ear/eye review by a musician wins.
- Compare to prior candidate on musical continuity, visual purpose, readability and truthful audience invitation; not just cut count.
- Private review only until release approval; no made-up crowd size, false urgency or fake performance content.

## Project test
Balkazar V5 had three photos held 0.72, 1.14 and 0.74 seconds; all failed the photo-pacing QA. V7 instead gives the horn photo 5.55–7.81 seconds and the crowd 10.47–12.74 seconds over a single 18.26s music excerpt. These are **specific to that music**, not general rules. Run `python check_photo_pacing.py Balkazar_V7_edit_manifest.json` when working with its manifest.

## Reference techniques (do not blindly install tool code)
- https://github.com/vidmoat/skills (music-montage, beat-sync, pacing; CC-BY-4.0 text)
- https://github.com/remotion-dev/skills (official Remotion production guides)
- https://github.com/WeftCut/WeftCut (optional reversible MCP timeline pilot)

[executed on device: ddjura-Summit-E14Evo-A12M (5f620238-a2f7-4969-98d2-f7fcd7ed04a5)]
