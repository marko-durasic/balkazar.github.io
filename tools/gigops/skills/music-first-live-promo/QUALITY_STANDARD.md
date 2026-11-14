# MusicOps — Balkazar V8 approved creative reference and future editing standard

**Decision date:** November 14, 2026 (Asia/Taipei)  
**Status:** Creatively approved by Balkazar's bandleader. **NOT APPROVED TO PUBLISH.**  
**Scope:** Reusable *minimum production-discipline reference* for new Balkazar videos, not a guarantee that every future clip will look or perform identically.

## What is preserved

- [Golden V8 MP4](https://drive.google.com/file/d/1EPomiH7bnvXZQvTLXBVCHZ_HpeW1s-Sq/view), sha256 `4827b9f36792b07738fadaa0ca463c6693ebcb4174e782a0a92bb89b1bdbf433`, 9,389,532 bytes. Google Drive name: `Balkazar_Dec03_2026_V8_APPROVED_CREATIVE_NOT_PUBLISHED.mp4`.
- [Exact original V8 edit manifest](https://drive.google.com/file/d/1SUECtvHaJLWGqrGyd1tQ4M3rl8ShhH7w/view) and [reproducible Python editor](https://drive.google.com/file/d/15KW4HgNe_TgHc4N7mijamcTiSGZ0qWEv/view) retained unchanged as historical provenance. The script uses the former `/mnt/data` runtime's paths and must be remapped for new work; a file path is not a live system integration.
- `v8_approved_reference.json` contains permanent Google Drive IDs, original/cutaway hashes, format, owner creative approval, protected singer windows, and the publication-denied boundary.
- `verify_music_first_reference.py` and `test_music_first_reference.py` test basic integrity and protect us from past errors. The optional source/audio comparison checks four windows in the approved file, but it is **not** a general music-quality detector.

## V1 → V8 lessons we should not lose

| Failure / observation | New rule |
| --- | --- |
| A downloaded compressed 360×640 clip looked blurry; export to 1080 was not true restoration | Select the best original file; inspect real resolution/bitrate; don't present interpolation as recovered detail |
| Captions disappeared beneath mobile player overlays | Keep titles and event CTA inside the safe region, preview on a phone and in 9:16 with controls |
| The first hook sounded exciting, then unnaturally reset to a slower passage | Lock one musically coherent recording before arranging images; avoid forced tempo jumps |
| Cutting right before the slow-to-fast payoff destroyed the performance | Listen to and retain the musical peak/phrase resolution; don't cut because a template hits 15/20 seconds |
| V5 used stills as 0.72/1.14/0.74-second flashes | Cutaways need identifiable story roles and enough dwell to be legible; length is context-dependent |
| V7 made the trombone solo photo 2.26s and hid the moment the singer started | Protect essential performer moments. If singer is on-camera during a vocal entrance, keep them visible; put horn photos where horns are musically meaningful |
| Removing stills altogether made the performance visually repetitive | Use purposeful *pokrivalice* (photo/B-roll overlay) while continuous main audio remains untouched |
| V8 combines a dancing hook, four ~1.1s meaningful stills, protected singing and unchanged live music | Approved reference for visual rhythm, artist identity, authenticity and restraint |

## V8 specific visual/music grammar (NOT universal timestamps)

- **0–2.18s**: genuinely energetic dancing in the first seconds; real live sound and band/event identity.
- **Audio 25.24–43.50s of original**: one uninterrupted 18.26-second excerpt. Only short start/end fades. Never reassemble scattered bars just to force a hook.
- **Singer window 4.38–8.557s on V8 timeline**: protected from cover images. In future edits, manually identify *new* vocalist's important visual moments; do not reuse V8 timestamps mechanically.
- **2.566–3.680**: 2022 band-and-audience image; **8.557–9.706**: 2022 drummer; **10.855–12.005**: 2022 brass; **13.514–14.675**: 2025 audience. Each insert had a specific purpose and modest ~2% movement. Historical years differ; never imply the material depicts one synchronized camera performance or the exact current lineup.
- **Format**: 720×1280 H.264/AAC, 30fps; do not inflate source sharpness claims. Keep text in the center/upper safe region.

## Reusable tool chain — what actually made V8

1. **Authenticated Google Drive**: locate genuine source files and retain original copies and manifest; label old 2022/2025 photos correctly, with recorded owner-attested promotional reuse permission and photographer/child privacy checks where required.
2. **FFmpeg/ffprobe**: verify resolution, codec, audio duration and continuous source excerpt; render H.264/AAC MP4. Use `+faststart` when supported and check platform compatibility; output metadata does not prove audience appeal.
3. **Python + OpenCV + NumPy + Pillow**: inspect images, compose 9:16 moving video under photo overlays, restrained 2% Ken Burns motion, blend ~100ms in/out, draw readable Traditional Chinese/English title cards. V8's original Python script contains these operations.
4. **Photo timing and music analysis**: choose visual cutaways against the *actual* original recording and musician feedback. `librosa`/audio-onset analysis was tested earlier in the pipeline, but a beat tracker does not know where vocals or meaningful phrases begin; don't blindly quantize every cut.
5. **Human director/bandleader feedback**: musical entrance visibility, dance-first hook, visual coherence, and purpose. This step made the difference between merely technically valid V5/V7 and accepted V8.
6. **Read-only visual and technical QA**: compare contact sheets and listen to final audio. Verify video/manifest/hash and music continuity; distinguish automated passes from aesthetic judgment.
7. **ClickUp + MusicOps Sheet**: preserve asset IDs, source links, edit decisions, owner creative approval, and **NOT PUBLISHED** status. Metricool must remain a separate user-approved publication workflow with dedupe receipts.

**Researched but NOT installed or used to produce V8:** WeftCut MCP editing UI, Montage AI, Remotion, PySceneDetect, OpenCLIP. Evaluate these only when a controlled pilot produces a better result for the same inputs without unexpected cost or privacy risk.

## Repeatable workflow for a new gig

1. Start from a confirmed artist/gig record. Find original footage; record source date, media rights and quality. Do not invent simultaneous angles.
2. Listen to a few strong music passages; pick one complete coherent phrase for the audio track. Mark where the performer needs to remain visually identifiable (especially singing and guitar solos).
3. Build the first 2–3 seconds from authentic crowd/dancing/performance action. If a cold open requires a musical discontinuity, reject it or select a different contiguous source phrase.
4. Prepare a **shot-purpose table**: why each photo/clip occurs, what music/lyric it relates to, enough visual readability, no obstruction of protected singer moments, no generic random inserts.
5. Use modest pans/zoom and a consistent transition language; cut on verified phrasing/action/selected strong beats, not a metronome.
6. Add accurate zh-Hant + English event CTA; phone-safe zone, test legibility without sound.
7. Export locally; run QA (`python -m unittest test_music_first_reference.py` and the golden integrity check where available), inspect frame contact sheet and listen to the first/last three seconds and every insert.
8. Compare the draft to the V8 reference for musical coherence, authenticity, picture purpose, readability and convincing invitation. **Do not claim the quality is objectively equal based on automated checks alone.** Get the bandleader's actual approval.
9. Only with separate explicit release authorization: select platform, caption, date/time, whether it replaces any existing draft, rights/identity boundaries, and verify actual provider receipts. No duplicates, no unsolicited contacts, no spending by default.
10. If a new issue occurs, record the *failure and fix* in the same skill/QA system (with a failing test first), not as another unconnected document or scheduler.

## Audience effectiveness — evidence vs assumptions

The short-form creative theory is: show real dancing and a memorable musical moment, communicate accurately when/where/free entry, link to the existing event, and invite a friend. Prior campaign data (Nov 10 Instagram: 124 views, 67 reached, 61.7% 3-second skip, 7.7s mean watch) was a small observational sample; **V8 has no published outcome yet**. We have not proved it improves attendance or is superior for every audience segment. Measure skip rate, duration-normalized watch time, meaningful shares, event actions and actual attendees after an owner-approved posting; do not manufacture attribution or statistical significance.

## Stop conditions

Do not proceed to publishing or paid editing service merely because a draft looks good. Stop if rights, privacy, publication authorization, audio continuity, authenticity, or gig details are unresolved. Preserve V8's reference SHA256; revisions become distinct creative versions, not silent overwrites.
