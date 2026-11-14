package gigops

// Cloud intake accepts exactly one typed, public-safe event and reconciles it
// against the existing private campaign state. The caller is authenticated
// by the existing *cloud* connector/OS session, never by a local laptop.
// This file creates no HTTP listener, uses no AI credential, and cannot
// contact providers or publish events/social posts.
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxCloudCandidateBytes int64 = 64 << 10

type CloudIntakeReceipt struct {
	SchemaVersion     int      `json:"schema_version"`
	Mode              string   `json:"mode"`
	GigID             string   `json:"gig_id"`
	BookingStatus     string   `json:"booking_status"`
	Revision          uint64   `json:"revision"`
	Change            string   `json:"change"`
	LocalStateWritten bool     `json:"local_state_written"`
	QueueChanged      bool     `json:"queue_changed"`
	QueueGeneration   uint64   `json:"queue_generation"`
	QueueItems        int      `json:"queue_items"`
	ProviderCalls     int      `json:"provider_calls"`
	PublicWrites      int      `json:"public_writes"`
	ExecutedJobs      int      `json:"executed_jobs"`
	Warnings          []string `json:"warnings,omitempty"`
}

func LoadCloudGigCandidate(stateDir, path string) (GigIntake, error) {
	if err := validCloudStateDir(stateDir); err != nil {
		return GigIntake{}, err
	}
	stateDir = filepath.Clean(stateDir)
	if !filepath.IsAbs(path) || filepath.Dir(filepath.Clean(path)) != filepath.Join(stateDir, "inbox") {
		return GigIntake{}, errors.New("candidate path must be a direct file in private cloud inbox")
	}
	inbox := filepath.Join(stateDir, "inbox")
	info, err := os.Lstat(inbox)
	if err != nil {
		return GigIntake{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return GigIntake{}, errors.New("cloud intake inbox must be private non-symlink directory")
	}
	file, err := os.Lstat(path)
	if err != nil {
		return GigIntake{}, err
	}
	if !file.Mode().IsRegular() || file.Mode()&os.ModeSymlink != 0 || file.Mode().Perm()&0077 != 0 ||
		file.Size() > maxCloudCandidateBytes {
		return GigIntake{}, errors.New("candidate must be private regular file of <=64 KiB")
	}
	reader, err := os.Open(path)
	if err != nil {
		return GigIntake{}, err
	}
	defer reader.Close()
	dec := json.NewDecoder(io.LimitReader(reader, maxCloudCandidateBytes+1))
	dec.DisallowUnknownFields()
	var g GigIntake
	if err := dec.Decode(&g); err != nil {
		return GigIntake{}, fmt.Errorf("invalid gig intake: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return GigIntake{}, errors.New("candidate has trailing JSON or exceeds length limit")
	}
	return g, nil
}

func PreviewCloudGigIntake(g GigIntake, now time.Time) (GigIntakePlan, error) {
	return PlanGigIntake(g, now)
}

// ApplyCloudGigIntake is deliberately PRIVATE and idempotent. Committing an
// intake writes campaign state and immediately reconciles queue metadata;
// a crash after the campaign write is recovered by the existing 5-minute timer.
func ApplyCloudGigIntake(stateDir string, g GigIntake, now time.Time) (CloudIntakeReceipt, error) {
	if err := validCloudStateDir(stateDir); err != nil {
		return CloudIntakeReceipt{}, err
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return CloudIntakeReceipt{}, err
	}
	result, err := ApplyGigIntakeFile(filepath.Join(stateDir, "campaigns.json"), g, now)
	if err != nil {
		return CloudIntakeReceipt{}, err
	}
	receipt := CloudIntakeReceipt{
		SchemaVersion: 1, Mode: "PRIVATE_INTAKE_NO_EXECUTION", GigID: g.GigID,
		BookingStatus: g.BookingStatus, Revision: result.Campaign.Revision,
		Change: result.Change, LocalStateWritten: result.LocalStateWritten,
		Warnings: append([]string(nil), result.Plan.Blockers...),
	}
	report, queueErr := RunCloudDryRun(stateDir, now)
	if queueErr != nil {
		receipt.Warnings = append(receipt.Warnings, "Private campaign intake persisted but queue refresh failed; the cloud timer will retry only metadata reconciliation")
		return receipt, fmt.Errorf("campaign persisted, queue refresh failed: %w", queueErr)
	}
	receipt.QueueChanged = report.QueueChanged
	receipt.QueueGeneration = report.QueueGeneration
	receipt.QueueItems = report.QueueItems
	receipt.ProviderCalls = report.ProviderCalls
	receipt.PublicWrites = report.PublicWrites
	receipt.ExecutedJobs = report.ExecutedJobs
	return receipt, nil
}
