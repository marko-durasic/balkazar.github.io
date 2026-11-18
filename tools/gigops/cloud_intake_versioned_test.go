package gigops

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testVersionedFile(t *testing.T, dir string, r VersionedCloudGigRequest) string {
	t.Helper()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, "confirmed-gig.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestVersionedCloudIntakeRejectsStaleReplayAndPreservesNewerGig(t *testing.T) {
	dir := tempCloudDir(t)
	g := sampleGig()
	req := VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 0, Gig: g}
	file := testVersionedFile(t, dir, req)
	parsed, err := LoadVersionedCloudGigCandidate(dir, file)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ApplyVersionedCloudGigIntake(dir, parsed, storeTime())
	if err != nil || first.Change != "created" || first.Revision != 1 || first.QueueItems != 10 {
		t.Fatalf("new gig not saved %+v %v", first, err)
	}
	replay, err := ApplyVersionedCloudGigIntake(dir, req, storeTime())
	if err != nil || replay.Change != "unchanged" || replay.QueueChanged {
		t.Fatalf("replay should be no-op %+v %v", replay, err)
	}
	moved := g
	moved.Start = "2026-12-04T20:00:00+08:00"
	moved.End = "2026-12-04T21:30:00+08:00"
	update := VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 1, Gig: moved}
	changed, err := ApplyVersionedCloudGigIntake(dir, update, storeTime())
	if err != nil || changed.Change != "revised" || changed.Revision != 2 || changed.QueueItems != 20 {
		t.Fatalf("reschedule failed %+v %v", changed, err)
	}
	stale := VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 1, Gig: g}
	if _, err := ApplyVersionedCloudGigIntake(dir, stale, storeTime()); err == nil || !strings.Contains(err.Error(), "stale intake") {
		t.Fatalf("stale old gig replay accepted: %v", err)
	}
	saved, err := LoadCampaignStore(filepath.Join(dir, "campaigns.json"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Campaigns[g.GigID].Revision != 2 || saved.Campaigns[g.GigID].Gig.Start != moved.Start {
		t.Fatal("stale replay reverted confirmed reschedule")
	}
}
func TestVersionedCloudIntakeCancellationAndMissingGig(t *testing.T) {
	dir := tempCloudDir(t)
	g := sampleGig()
	if _, err := ApplyVersionedCloudGigIntake(dir, VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 1, Gig: g}, storeTime()); err == nil {
		t.Fatal("new gig with nonzero expected revision accepted")
	}
	created, err := ApplyVersionedCloudGigIntake(dir, VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 0, Gig: g}, storeTime())
	if err != nil || created.Revision != 1 {
		t.Fatal(err)
	}
	g.BookingStatus = "cancelled"
	cancelled, err := ApplyVersionedCloudGigIntake(dir, VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 1, Gig: g}, storeTime())
	if err != nil || cancelled.Change != "cancelled" || cancelled.PublicWrites != 0 {
		t.Fatalf("cancellation failed %+v %v", cancelled, err)
	}
}
func TestVersionedCandidateInboxAndUnknownFields(t *testing.T) {
	dir := tempCloudDir(t)
	v := VersionedCloudGigRequest{IntakeVersion: 1, ExpectedRevision: 0, Gig: sampleGig()}
	file := testVersionedFile(t, dir, v)
	got, err := LoadVersionedCloudGigCandidate(dir, file)
	if err != nil || got.Gig.GigID != v.Gig.GigID {
		t.Fatal(err)
	}
	if _, err := LoadVersionedCloudGigCandidate(dir, filepath.Join(dir, "elsewhere.json")); err == nil {
		t.Fatal("accepted non-inbox")
	}
	if err := os.WriteFile(file, []byte("{\"intake_version\":1,\"expected_revision\":0,\"gig\":{},\"authorization\":true}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVersionedCloudGigCandidate(dir, file); err == nil {
		t.Fatal("unknown model-generated approval field accepted")
	}
	if err := os.WriteFile(file, []byte("{\"intake_version\":99,\"expected_revision\":0,\"gig\":{}}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVersionedCloudGigCandidate(dir, file); err == nil {
		t.Fatal("wrong intake schema accepted")
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVersionedCloudGigCandidate(dir, file); err == nil {
		t.Fatal("loose perms accepted")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVersionedCloudGigCandidate(dir, file); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing input not detected")
	}
}
