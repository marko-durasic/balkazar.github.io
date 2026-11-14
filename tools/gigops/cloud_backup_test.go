package gigops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleBackupState(t *testing.T) (string, time.Time) {
	t.Helper()
	dir := tempCloudDir(t)
	now := storeTime()
	if _, err := ApplyCloudGigIntake(dir, sampleGig(), now); err != nil {
		t.Fatal(err)
	}
	return dir, now
}
func TestCloudBackupPreparesWithinFreeStandardTierAndRestores(t *testing.T) {
	dir, now := sampleBackupState(t)
	blob, report, err := PrepareCloudBackup(dir, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(blob) == 0 || len(blob) > 4000 || report.EncodedBytes != len(blob) ||
		report.Mode != "PREPARED_OFFHOST_BACKUP_NOT_UPLOADED" || report.Campaigns != 1 || report.QueueItems != 10 {
		t.Fatalf("wrong compact backup %+v; bytes=%d", report, len(blob))
	}
	if report.PublicWrites != 0 || report.ProviderCalls != 0 {
		t.Fatal("preparing backup attempted network/provider write")
	}
	decoded, err := DecodeCloudBackup(blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Files) != 3 {
		t.Fatal("backup lost files")
	}
	target := filepath.Join(t.TempDir(), "restore")
	recovered, err := RestoreCloudBackup(blob, target, now.Add(5*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Campaigns != 1 || recovered.QueueItems != 10 || recovered.PublicWrites != 0 {
		t.Fatalf("restore result invalid %+v", recovered)
	}
	for _, name := range []string{"campaigns.json", "queue.json", "heartbeat.json"} {
		st, err := os.Stat(filepath.Join(target, name))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0600 {
			t.Fatalf("restored %s not protected", name)
		}
	}
	next, err := RunCloudDryRun(target, now.Add(5*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if next.QueueChanged || next.QueueGeneration != 1 || next.Campaigns != 1 || next.PublicWrites != 0 {
		t.Fatalf("restored queue not resumable %+v", next)
	}
}
func TestCloudBackupRejectsCorruptionAndRefusesOverwrite(t *testing.T) {
	dir, now := sampleBackupState(t)
	blob, _, err := PrepareCloudBackup(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	var env CloudBackupEnvelope
	if err := json.Unmarshal([]byte(blob), &env); err != nil {
		t.Fatal(err)
	}
	env.SHA256 = strings.Repeat("e", 64)
	corrupt, _ := json.Marshal(env)
	if _, err := DecodeCloudBackup(string(corrupt)); err == nil {
		t.Fatal("accepted corrupted digest")
	}
	target := filepath.Join(t.TempDir(), "occupied")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "important.txt")
	if err := os.WriteFile(marker, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreCloudBackup(blob, target, now); err == nil {
		t.Fatal("restore overwrote an existing directory")
	}
	saved, err := os.ReadFile(marker)
	if err != nil || string(saved) != "preserve" {
		t.Fatal("restore destroyed destination data")
	}
}
func TestCloudBackupRejectsStaleOrInconsistentSourceState(t *testing.T) {
	dir, now := sampleBackupState(t)
	if _, _, err := PrepareCloudBackup(dir, now.Add(time.Hour)); err == nil {
		t.Fatal("stale heartbeat accepted")
	}
	// Apply new gig revision without the queue materializer; backup must block.
	g := sampleGig()
	g.Start = "2026-12-04T20:00:00+08:00"
	g.End = "2026-12-04T21:30:00+08:00"
	if _, err := ApplyGigIntakeFile(filepath.Join(dir, "campaigns.json"), g, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareCloudBackup(dir, now.Add(2*time.Minute)); err == nil {
		t.Fatal("inconsistent queue backed up")
	}
}
func TestBackupDecodeBlocksOversizeAndWrongFields(t *testing.T) {
	if _, err := DecodeCloudBackup(strings.Repeat("z", 4001)); err == nil {
		t.Fatal("oversize backup accepted")
	}
	if _, err := DecodeCloudBackup("{}"); err == nil {
		t.Fatal("empty envelope accepted")
	}
	if _, err := DecodeCloudBackup("{\"schema_version\":1,\"sha256\":\"abc\",\"gzip_base64\":\"\"}"); err == nil {
		t.Fatal("invalid digest accepted")
	}
}
func TestCloudBackupDoesNotRestoreSecretsOrTouchExternalProvider(t *testing.T) {
	dir, now := sampleBackupState(t)
	blob, _, err := PrepareCloudBackup(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(blob, "private_key") || strings.Contains(blob, "access_token") {
		t.Fatal("backup envelope includes secret-looking cleartext")
	}
	target := filepath.Join(t.TempDir(), "restored")
	if _, err := RestoreCloudBackup(blob, target, now); err != nil {
		t.Fatal(err)
	}
	report, err := CheckCloudDryRunHealth(target, now.Add(time.Minute), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if report.ProviderCalls != 0 || report.ExecutedJobs != 0 || report.PublicWrites != 0 {
		t.Fatalf("restore unexpectedly executed work %+v", report)
	}
}
