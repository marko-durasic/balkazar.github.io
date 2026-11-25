package gigops

// Versioned cloud intake is the only supported production-facing ingress.
// The optimistic revision is compared while holding the campaign file lock,
// preventing stale chat messages from undoing confirmed gig updates.
import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type VersionedCloudGigRequest struct {
	IntakeVersion    int       `json:"intake_version"`
	ExpectedRevision uint64    `json:"expected_revision"` // 0 when creating a new gig
	Gig              GigIntake `json:"gig"`
}

func LoadVersionedCloudGigCandidate(dir, path string) (VersionedCloudGigRequest, error) {
	if err := validCloudStateDir(dir); err != nil {
		return VersionedCloudGigRequest{}, err
	}
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(path) || filepath.Dir(filepath.Clean(path)) != filepath.Join(dir, "inbox") {
		return VersionedCloudGigRequest{}, errors.New("versioned candidate must be a direct file in private cloud inbox")
	}
	inbox, err := os.Lstat(filepath.Join(dir, "inbox"))
	if err != nil {
		return VersionedCloudGigRequest{}, err
	}
	if !inbox.IsDir() || inbox.Mode()&os.ModeSymlink != 0 || inbox.Mode().Perm()&0077 != 0 {
		return VersionedCloudGigRequest{}, errors.New("cloud intake inbox must be private non-symlink dir")
	}
	st, err := os.Lstat(path)
	if err != nil {
		return VersionedCloudGigRequest{}, err
	}
	if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 ||
		st.Size() > maxCloudCandidateBytes {
		return VersionedCloudGigRequest{}, errors.New("candidate must be a private regular file <=64KiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return VersionedCloudGigRequest{}, err
	}
	defer f.Close()
	var request VersionedCloudGigRequest
	dec := json.NewDecoder(io.LimitReader(f, maxCloudCandidateBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&request); err != nil {
		return VersionedCloudGigRequest{}, fmt.Errorf("invalid versioned gig intake: %w", err)
	}
	var tail any
	if err := dec.Decode(&tail); err != io.EOF {
		return VersionedCloudGigRequest{}, errors.New("extra JSON in versioned gig candidate")
	}
	if request.IntakeVersion != 1 {
		return VersionedCloudGigRequest{}, errors.New("unsupported intake version")
	}
	return request, nil
}
func ApplyVersionedCloudGigIntake(dir string, request VersionedCloudGigRequest, now time.Time) (CloudIntakeReceipt, error) {
	if request.IntakeVersion != 1 {
		return CloudIntakeReceipt{}, errors.New("unsupported intake version")
	}
	if err := validCloudStateDir(dir); err != nil {
		return CloudIntakeReceipt{}, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return CloudIntakeReceipt{}, err
	}
	g := request.Gig
	result, err := ApplyGigIntakeIfRevision(filepath.Join(dir, "campaigns.json"), g, request.ExpectedRevision, now)
	if err != nil {
		return CloudIntakeReceipt{}, err
	}
	receipt := CloudIntakeReceipt{
		SchemaVersion: 1, Mode: "PRIVATE_VERSIONED_INTAKE_NO_EXECUTION",
		GigID: g.GigID, BookingStatus: g.BookingStatus, Revision: result.Campaign.Revision,
		Change: result.Change, LocalStateWritten: result.LocalStateWritten,
		Warnings: append([]string(nil), result.Plan.Blockers...),
	}
	report, err := RunCloudDryRun(dir, now)
	if err != nil {
		receipt.Warnings = append(receipt.Warnings, "Campaign was persisted; queue refresh failed; five-minute cloud timer will reconcile safely")
		return receipt, fmt.Errorf("versioned campaign persisted but queue refresh failed: %w", err)
	}
	receipt.QueueChanged = report.QueueChanged
	receipt.QueueGeneration = report.QueueGeneration
	receipt.QueueItems = report.QueueItems
	receipt.ProviderCalls = report.ProviderCalls
	receipt.PublicWrites = report.PublicWrites
	receipt.ExecutedJobs = report.ExecutedJobs
	return receipt, nil
}
