#!/usr/bin/env python3
"""MusicOps music-first promo QA. Technical gates are NOT artistic approval.

Use the approved V8 as a reproducible golden reference, without blocking
future innovative edits solely because their durations or style differ.

Usage:
  python verify_music_first_reference.py --reference v8_approved_reference.json
  python verify_music_first_reference.py --reference v8_approved_reference.json \
      --video /path/approved-v8.mp4 --source /path/43-second-source.mp4
"""
import argparse
from array import array
import hashlib
import json
import math
import shutil
import subprocess
from pathlib import Path


def load_reference(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def audit_reference(ref):
    errors = []
    video = ref.get("video", {})
    source = ref.get("main_source", {})
    approval = ref.get("approval", {})
    if approval.get("creative") != "owner_approved":
        errors.append("Missing explicit creative approval")
    if approval.get("publication") != "NOT_AUTHORIZED":
        errors.append("Creative approval must not become publication approval")
    duration = float(video.get("duration_seconds", 0))
    if duration <= 0:
        errors.append("Invalid video duration")
    if not source.get("audio_continuous"):
        errors.append("Reference music must be continuous")
    audio_seconds = float(source.get("audio_end_seconds", 0)) - float(source.get("audio_start_seconds", 0))
    if abs(audio_seconds - duration) > 0.08:
        errors.append("Reference audio range does not match video duration")
    protected = ref.get("protected_moments", [])
    for p in protected:
        if not (0 <= p["start"] < p["end"] <= duration + .01):
            errors.append("Protected performance moment is outside video timeline")
    prev_end = 0
    cutaways = ref.get("cutaways", [])
    if not cutaways:
        errors.append("No cutaway storytelling plan supplied")
    for cut in cutaways:
        st, en = float(cut["start"]), float(cut["end"])
        if not (0 <= st < en <= duration + .01):
            errors.append(f"Out-of-range cutaway: {cut.get('key')}")
        if st < prev_end - .001:
            errors.append(f"Overlapping or unsorted cutaways: {cut.get('key')}")
        prev_end = en
        if not cut.get("story_role", "").strip() or not cut.get("drive_id") or not cut.get("source_year"):
            errors.append(f"Missing purpose/provenance: {cut.get('key')}")
        for p in protected:
            if st < p["end"] - .001 and en > p["start"] + .001:
                errors.append(f"Cutaway {cut.get('key')} hides protected lead moment")
    if not ref.get("quality_notes", {}).get("data_policy"):
        errors.append("Source/rights/privacy guidance missing")
    return errors


def sha256_file(path):
    with Path(path).open("rb") as stream:
        digest = hashlib.sha256()
        for b in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(b)
    return digest.hexdigest()


def ffprobe(path):
    if not shutil.which("ffprobe"):
        raise RuntimeError("ffprobe not installed")
    raw = subprocess.check_output([
        "ffprobe", "-v", "error", "-show_entries",
        "format=duration:stream=codec_type,codec_name,width,height,avg_frame_rate",
        "-of", "json", str(path)], text=True)
    result = json.loads(raw)
    streams = {s["codec_type"]: s for s in result["streams"] if s.get("codec_type") in ("audio", "video")}
    return result, streams


def audit_approved_file(ref, path):
    errors = []
    video = ref["video"]
    p = Path(path)
    if not p.is_file():
        return ["Approved reference file not found"]
    if p.stat().st_size != video["bytes"]:
        errors.append("Approved video byte size changed")
    if sha256_file(p) != video["sha256"]:
        errors.append("Approved video SHA-256 changed — reference is not immutable")
    try:
        probe, streams = ffprobe(p)
        av = streams.get("video", {})
        aa = streams.get("audio", {})
        if (av.get("width"), av.get("height")) != (video["width"], video["height"]):
            errors.append("Approved frame size mismatch")
        if (av.get("codec_name"), aa.get("codec_name")) != (video["video_codec"], video["audio_codec"]):
            errors.append("Approved codec mismatch")
        if abs(float(probe["format"]["duration"]) - video["duration_seconds"]) > .05:
            errors.append("Approved duration mismatch")
        n, d = map(int, av.get("avg_frame_rate", "0/1").split("/"))
        if not d or abs(n/d-video["fps"]) > 0.001:
            errors.append("Approved frame rate mismatch")
    except (KeyError, ValueError, RuntimeError, subprocess.CalledProcessError) as exc:
        errors.append("ffprobe check failed: " + str(exc))
    return errors


def decode_mono(path, start=None, duration=None, sample_rate=8000):
    """Decode audio using ffmpeg to mono 16-bit PCM; stdlib only."""
    if not shutil.which("ffmpeg"):
        raise RuntimeError("ffmpeg not installed")
    cmd = ["ffmpeg", "-hide_banner", "-v", "error"]
    if start is not None:
        cmd.extend(["-ss", str(start)])
    cmd.extend(["-i", str(path)])
    if duration is not None:
        cmd.extend(["-t", str(duration)])
    cmd.extend(["-vn", "-ac", "1", "-ar", str(sample_rate), "-f", "s16le", "pipe:1"])
    proc = subprocess.run(cmd, capture_output=True, check=True)
    pcm = array("h")
    pcm.frombytes(proc.stdout)
    if __import__("sys").byteorder != "little":
        pcm.byteswap()
    return pcm


def cosine_similarity(xs, ys):
    product = sum(float(a)*b for a, b in zip(xs, ys))
    x2 = sum(float(a)*a for a in xs)
    y2 = sum(float(b)*b for b in ys)
    return product / math.sqrt(x2*y2) if x2*y2 else 0.0


def audit_audio_continuity(ref, video_path, source_path):
    """Golden V8 fingerprint: compare multiple musical windows at exact source offsets.

    This test is specific to V8 and should NOT be imposed on new recordings.
    """
    t0 = ref["main_source"]["audio_start_seconds"]
    duration = ref["video"]["duration_seconds"]
    expected = decode_mono(source_path, start=t0, duration=duration)
    actual = decode_mono(video_path)
    errors = []
    for start_s in (2.0, 7.0, 12.0, 16.0):
        i = round(start_s*8000)
        n = 8000
        if min(len(expected),len(actual)) < i+n:
            errors.append(f"Audio too short to check {start_s}s")
            continue
        similarity = cosine_similarity(expected[i:i+n],actual[i:i+n])
        if similarity < .97:
            errors.append(f"Audio discontinuity/other recording detected at {start_s}s (similarity={similarity:.3f})")
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference", required=True)
    parser.add_argument("--video", help="Exact approved V8 file; optional full-integrity check")
    parser.add_argument("--source", help="Exact archived source file, for optional audio fingerprint check")
    opts = parser.parse_args()
    ref = load_reference(opts.reference)
    errors = audit_reference(ref)
    if opts.video:
        errors.extend(audit_approved_file(ref, opts.video))
    if opts.source and opts.video:
        errors.extend(audit_audio_continuity(ref, opts.video, opts.source))
    for err in errors:
        print("FAIL:", err)
    if errors:
        raise SystemExit(1)
    print("PASS: approved V8 reference constraints", "and exact file/audio" if opts.video else "(metadata only)")
    print("NOTE: subjective editing quality and public publishing approvals require separate review")


if __name__ == "__main__":
    main()
