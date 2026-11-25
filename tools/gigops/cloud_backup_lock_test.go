package gigops

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func holdCloudBackupTestLock(t *testing.T, dir, name string) func() {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		t.Fatal(err)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }
}

func TestCloudBackupRefusesConcurrentTimerQueueWrite(t *testing.T) {
	dir, now := sampleBackupState(t)
	release := holdCloudBackupTestLock(t, dir, ".cloud-worker.lock")
	_, _, err := PrepareCloudObjectBackup(dir, now.Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "busy") {
		release()
		t.Fatalf("backup raced active cloud timer: %v", err)
	}
	release()
	blob, report, err := PrepareCloudObjectBackup(dir, now.Add(time.Minute))
	if err != nil || blob == "" || report.Campaigns != 1 {
		t.Fatalf("failed to resume after worker released lock: %v", err)
	}
}
func TestCloudBackupRefusesConcurrentGigRevision(t *testing.T) {
	dir, now := sampleBackupState(t)
	release := holdCloudBackupTestLock(t, dir, "campaigns.json.lock")
	_, _, err := PrepareCloudBackup(dir, now.Add(time.Minute))
	if err == nil || !strings.Contains(err.Error(), "busy") {
		release()
		t.Fatalf("snapshot raced gig revision: %v", err)
	}
	release()
	// The first lock must be released even though acquiring the second failed.
	if _, err := RunCloudDryRun(dir, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("backup leaked cloud lock after failed campaign lock: %v", err)
	}
	if _, _, err := PrepareCloudBackup(dir, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
}
func TestCloudBackupRefusesSymlinkedLock(t *testing.T) {
	dir, now := sampleBackupState(t)
	original := filepath.Join(dir, "campaigns.json.lock")
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	sensitive := filepath.Join(t.TempDir(), "test-target")
	if err := os.WriteFile(sensitive, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sensitive, original); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareCloudObjectBackup(dir, now); err == nil {
		t.Fatal("accepted symlink lock in private snapshot path")
	}
	b, err := os.ReadFile(sensitive)
	if err != nil || string(b) != "untouched" {
		t.Fatal("unsafe snapshot lock modified target")
	}
}
func TestCloudBackupReadOnlyAndStableAgainstExistingState(t *testing.T) {
	dir, now := sampleBackupState(t)
	files := []string{"campaigns.json", "queue.json", "heartbeat.json"}
	before := map[string]string{}
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = string(b)
	}
	encoded, report, err := PrepareCloudObjectBackup(dir, now.Add(time.Minute))
	if err != nil || report.PublicWrites != 0 || report.ProviderCalls != 0 {
		t.Fatal(err)
	}
	_, err = DecodeCloudObjectBackup(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != before[name] {
			t.Fatalf("read-only backup changed %s", name)
		}
	}
}
