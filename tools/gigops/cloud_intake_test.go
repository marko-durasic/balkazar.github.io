package gigops

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cloudTestCandidate(t *testing.T, dir string, g GigIntake) string {
	t.Helper()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, "gig.json")
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestCloudIntakePreviewDoesNotWrite(t *testing.T) {
	dir := tempCloudDir(t)
	path := cloudTestCandidate(t, dir, sampleGig())
	g, err := LoadCloudGigCandidate(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := PreviewCloudGigIntake(g, storeTime())
	if err != nil {
		t.Fatal(err)
	}
	if !p.BookingReady || p.PublicWrites != 0 {
		t.Fatal("confirmed preview invalid")
	}
	if _, err := os.Stat(filepath.Join(dir, "campaigns.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview unexpectedly wrote campaign state")
	}
}
func TestCloudIntakeApplyIdempotenceAndQueueRecovery(t *testing.T) {
	dir := tempCloudDir(t)
	g := sampleGig()
	path := cloudTestCandidate(t, dir, g)
	supplied, err := LoadCloudGigCandidate(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ApplyCloudGigIntake(dir, supplied, storeTime())
	if err != nil {
		t.Fatal(err)
	}
	if first.Change != "created" || first.Revision != 1 || first.QueueItems != 10 || first.ProviderCalls != 0 || first.PublicWrites != 0 || first.ExecutedJobs != 0 {
		t.Fatalf("unexpected cloud intake receipt %+v", first)
	}
	before, err := os.ReadFile(filepath.Join(dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := ApplyCloudGigIntake(dir, supplied, storeTime().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if again.Change != "unchanged" || again.QueueChanged || again.QueueGeneration != 1 {
		t.Fatalf("duplicate gig changed queue %+v", again)
	}
	after, err := os.ReadFile(filepath.Join(dir, "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("duplicate intake altered queue bytes")
	}
	// Simulate a crash after campaign is saved but before queue update:
	g.Start = "2026-12-04T20:00:00+08:00"
	g.End = "2026-12-04T21:30:00+08:00"
	if _, err := ApplyGigIntakeFile(filepath.Join(dir, "campaigns.json"), g, storeTime().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	recovered, err := RunCloudDryRun(dir, storeTime().Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Superseded != 10 || recovered.QueueItems != 20 || recovered.QueueGeneration != 2 {
		t.Fatalf("cloud timer could not recover pending gig update %+v", recovered)
	}
}
func TestCloudIntakeCancellationAndDuplicateRejection(t *testing.T) {
	dir := tempCloudDir(t)
	g := sampleGig()
	first, err := ApplyCloudGigIntake(dir, g, storeTime())
	if err != nil {
		t.Fatal(err)
	}
	if first.QueueItems != 10 {
		t.Fatal("unexpected initial queue")
	}
	other := g
	other.GigID = "balkazar-duplicate-id"
	if _, err := ApplyCloudGigIntake(dir, other, storeTime()); err == nil {
		t.Fatal("duplicate gig was accepted")
	}
	g.BookingStatus = "cancelled"
	cancel, err := ApplyCloudGigIntake(dir, g, storeTime().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if cancel.Change != "cancelled" || cancel.QueueItems != 12 || cancel.PublicWrites != 0 {
		t.Fatalf("cancellation incorrectly processed %+v", cancel)
	}
}
func TestCloudIntakeRejectsNonInboxAndLooseFiles(t *testing.T) {
	dir := tempCloudDir(t)
	valid := cloudTestCandidate(t, dir, sampleGig())
	if _, err := LoadCloudGigCandidate(dir, valid); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "outside.json")
	bytes, err := os.ReadFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fake, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCloudGigCandidate(dir, fake); err == nil {
		t.Fatal("accepted gig outside protected inbox")
	}
	link := filepath.Join(dir, "inbox", "symlink.json")
	if err := os.Symlink(fake, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCloudGigCandidate(dir, link); err == nil {
		t.Fatal("accepted symlink candidate")
	}
	if err := os.Chmod(valid, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCloudGigCandidate(dir, valid); err == nil {
		t.Fatal("accepted readable-by-world input")
	}
	if err := os.Chmod(valid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(valid, []byte("{bad json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCloudGigCandidate(dir, valid); err == nil {
		t.Fatal("accepted malformed JSON")
	}
}
func TestCloudIntakeRejectsUnknownFieldsAndTenant(t *testing.T) {
	dir := tempCloudDir(t)
	path := cloudTestCandidate(t, dir, sampleGig())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\n{}")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCloudGigCandidate(dir, path); err == nil {
		t.Fatal("accepted appended second record")
	}
	g := sampleGig()
	g.Artist = "Fuzzy Wall"
	if _, err := ApplyCloudGigIntake(dir, g, time.Now()); err == nil {
		t.Fatal("cross-artist write accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "campaigns.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rejected tenant was stored")
	}
	g = sampleGig()
	g.GigID = ""
	p, err := PreviewCloudGigIntake(g, storeTime())
	if err != nil {
		t.Fatal(err)
	}
	if p.BookingReady || !strings.Contains(strings.Join(p.Blockers, " "), "gig_id") {
		t.Fatal("missing stable gig identity not blocked")
	}
}
