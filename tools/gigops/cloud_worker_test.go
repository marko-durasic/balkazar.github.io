package gigops

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func tempCloudDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "musicops")
}
func TestCloudWorkerEmptyBootAndIdempotentRun(t *testing.T) {
	dir := tempCloudDir(t)
	now := storeTime()
	first, err := RunCloudDryRun(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if !first.QueueChanged || first.QueueGeneration != 1 || first.QueueItems != 0 ||
		first.PublicWrites != 0 || first.ProviderCalls != 0 || first.ExecutedJobs != 0 {
		t.Fatalf("unexpected first dry-run %+v", first)
	}
	queuePath := filepath.Join(dir, "queue.json")
	before, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunCloudDryRun(dir, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	if second.QueueChanged || second.QueueGeneration != first.QueueGeneration || string(before) != string(after) {
		t.Fatal("no-op second run changed durable queue")
	}
	info, err := os.Stat(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("queue is not 0600: %v", info.Mode().Perm())
	}
	directory, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if directory.Mode().Perm() != 0700 {
		t.Fatal("state directory is not private 0700")
	}
	if _, err := CheckCloudDryRunHealth(dir, now.Add(2*time.Minute), 5*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckCloudDryRunHealth(dir, now.Add(40*time.Minute), 5*time.Minute); err == nil {
		t.Fatal("stale heartbeat accepted as healthy")
	}
}

func TestCloudWorkerCampaignRescheduleAndCancellation(t *testing.T) {
	dir := tempCloudDir(t)
	g := sampleGig()
	store := NewCampaignStore()
	now := storeTime()
	if _, err := store.ReconcileGig(g, now); err != nil {
		t.Fatal(err)
	}
	if err := SaveCampaignStore(filepath.Join(dir, "campaigns.json"), store); err != nil {
		t.Fatal(err)
	}
	first, err := RunCloudDryRun(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.QueueItems != 10 || first.PrivateReady != 1 || first.PublicWrites != 0 || first.ProviderCalls != 0 {
		t.Fatalf("new campaign queue incorrect %+v", first)
	}
	_, err = RunCloudDryRun(dir, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	g.Start = "2026-12-04T20:00:00+08:00"
	g.End = "2026-12-04T21:30:00+08:00"
	if _, err := store.ReconcileGig(g, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := SaveCampaignStore(filepath.Join(dir, "campaigns.json"), store); err != nil {
		t.Fatal(err)
	}
	changed, err := RunCloudDryRun(dir, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if changed.QueueItems != 20 || changed.Superseded != 10 || !changed.QueueChanged || changed.QueueGeneration != 2 {
		t.Fatalf("reschedule lost old revision or duplicate keys %+v", changed)
	}
	g.BookingStatus = "cancelled"
	if _, err := store.ReconcileGig(g, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := SaveCampaignStore(filepath.Join(dir, "campaigns.json"), store); err != nil {
		t.Fatal(err)
	}
	cancelled, err := RunCloudDryRun(dir, now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.QueueItems != 22 || cancelled.Superseded != 20 || cancelled.Blocked < 2 || cancelled.PrivateReady != 0 {
		t.Fatalf("cancelled gig left executable work or lost history %+v", cancelled)
	}
	items, err := CloudQueueSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 22 {
		t.Fatal("queue snapshot changed length")
	}
	for _, item := range items {
		if item.Revision == 3 && item.DesiredState != "external-blocked" {
			t.Fatalf("cancelled event advanced network tasks: %+v", item)
		}
	}
}

func TestCloudWorkerNeverOverwritesCorruptQueue(t *testing.T) {
	dir := tempCloudDir(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "queue.json")
	payload := []byte("{ invalid queue")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RunCloudDryRun(dir, storeTime()); err == nil {
		t.Fatal("accepted corrupt durable queue")
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != string(payload) {
		t.Fatal("overwrote corrupt queue, losing forensics")
	}
}
func TestCloudWorkerRejectsStateSymlinksAndLooseModes(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "musicops")
	target := filepath.Join(parent, "elsewhere")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := RunCloudDryRun(dir, storeTime()); err == nil {
		t.Fatal("symlink state dir accepted")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := RunCloudDryRun(dir, storeTime()); err == nil {
		t.Fatal("public state dir accepted")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := RunCloudDryRun(dir, storeTime()); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(dir, "queue.json")
	if err := os.Chmod(queue, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunCloudDryRun(dir, storeTime()); err == nil {
		t.Fatal("loose file permissions accepted")
	}
}
func TestCloudWorkerNonblockingLock(t *testing.T) {
	dir := tempCloudDir(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, ".cloud-worker.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	if _, err := RunCloudDryRun(dir, storeTime()); err == nil {
		t.Fatal("concurrent worker accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "heartbeat.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("overlapping worker touched heartbeat")
	}
}
func TestCloudWorkerCannotClaimProviderExecution(t *testing.T) {
	r, err := RunCloudDryRun(tempCloudDir(t), storeTime())
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != "DRY_RUN_NO_EXECUTION" || r.ExecutedJobs != 0 || r.ProviderCalls != 0 || r.PublicWrites != 0 {
		t.Fatal("dry-run worker misreported side effects")
	}
	_, err = CheckCloudDryRunHealth(tempCloudDir(t), time.Now(), time.Minute)
	if err == nil {
		t.Fatal("empty unrelated dir claims healthy")
	}
}
