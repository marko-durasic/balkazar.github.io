package gigops

// Cloud dry-run materializer: no HTTP, no provider adapter, no credentials.
// Reads the private CampaignStore and maintains a bounded idempotent queue
// snapshot plus a heartbeat. Never executes any campaign job or social write.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const cloudQueueVersion = 1
const cloudFileLimit = 2 << 20

type CloudQueueItem struct {
	Key          string `json:"key"`
	GigID        string `json:"gig_id"`
	Revision     uint64 `json:"revision"`
	JobID        string `json:"job_id"`
	Execution    string `json:"execution"`
	DesiredState string `json:"desired_state"`
	FirstSeenAt  string `json:"first_seen_at"`
}
type CloudQueue struct {
	SchemaVersion int                       `json:"schema_version"`
	Generation    uint64                    `json:"generation"`
	UpdatedAt     string                    `json:"updated_at"`
	Items         map[string]CloudQueueItem `json:"items"`
}
type CloudDryRunReport struct {
	SchemaVersion   int    `json:"schema_version"`
	Mode            string `json:"mode"`
	CheckedAt       string `json:"checked_at"`
	QueueGeneration uint64 `json:"queue_generation"`
	QueueChanged    bool   `json:"queue_changed"`
	Campaigns       int    `json:"campaigns"`
	QueueItems      int    `json:"queue_items"`
	PrivateReady    int    `json:"private_ready"`
	Blocked         int    `json:"blocked"`
	HumanHandoffs   int    `json:"human_handoffs"`
	Superseded      int    `json:"superseded"`
	ExecutedJobs    int    `json:"executed_jobs"`
	ProviderCalls   int    `json:"provider_calls"`
	PublicWrites    int    `json:"public_writes"`
}

func emptyCloudQueue() CloudQueue {
	return CloudQueue{SchemaVersion: cloudQueueVersion, Items: make(map[string]CloudQueueItem)}
}
func cloudJobKey(gig string, revision uint64, job string) string {
	bytes := []byte(fmt.Sprintf("%s:%d:%s", gig, revision, job))
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
func cloudDesiredState(job CampaignJobRecord) string {
	if job.Execution == "human-physical" || job.State == "human-handoff" {
		return "human-handoff"
	}
	if job.Execution != "automatic-draft" {
		return "external-blocked"
	}
	switch job.State {
	case "ready":
		return "private-ready"
	case "private-complete":
		return "private-complete-observed"
	case "needs-verification":
		return "verification-blocked"
	default:
		return "review-blocked"
	}
}
func validCloudStateDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return errors.New("absolute cloud state directory required")
	}
	dir = filepath.Clean(dir)
	if dir == "/" || dir == os.TempDir() {
		return errors.New("refusing broad cloud state directory")
	}
	if err := checkCampaignFilePath(filepath.Join(dir, "campaigns.json")); err != nil {
		return err
	}
	if st, err := os.Lstat(dir); err == nil {
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
			return errors.New("cloud state directory must be private 0700, non-symlink")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func privatePath(path string) error {
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("private state path %s must be regular 0600 (not symlink)", filepath.Base(path))
	}
	if st.Size() > cloudFileLimit {
		return errors.New("private state file exceeds 2 MiB limit")
	}
	return nil
}
func readPrivateJSON(path string, value any) error {
	if err := privatePath(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, cloudFileLimit+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var tail any
	if err := decoder.Decode(&tail); err != io.EOF {
		return errors.New("private JSON contains extra data")
	}
	return nil
}
func savePrivateJSON(path string, value any) error {
	if err := privatePath(path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(data)+1 > cloudFileLimit {
		return errors.New("private JSON would exceed 2 MiB")
	}
	tmp, err := os.CreateTemp(dir, ".musicops-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, byte(10))); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	// fsync the containing directory as well as the temporary file, so the
	// atomic rename is durable across an ordinary power loss on ext4/EBS.
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func loadCloudQueue(path string) (CloudQueue, error) {
	var q CloudQueue
	err := readPrivateJSON(path, &q)
	if errors.Is(err, os.ErrNotExist) {
		return emptyCloudQueue(), nil
	}
	if err != nil {
		return CloudQueue{}, err
	}
	if q.SchemaVersion != cloudQueueVersion || q.Items == nil || len(q.Items) > 10000 {
		return CloudQueue{}, errors.New("unsupported or oversized cloud queue")
	}
	for k, item := range q.Items {
		if !storedFingerprintRE.MatchString(k) || item.Key != k ||
			!durableGigID.MatchString(item.GigID) || item.Revision == 0 || item.JobID == "" ||
			!allowed(item.DesiredState, "private-ready", "private-complete-observed", "verification-blocked",
				"review-blocked", "waiting-dependency", "external-blocked", "human-handoff", "superseded") ||
			k != cloudJobKey(item.GigID, item.Revision, item.JobID) {
			return CloudQueue{}, errors.New("invalid cloud queue entry")
		}
		if _, err := time.Parse(time.RFC3339, item.FirstSeenAt); err != nil {
			return CloudQueue{}, errors.New("queue item missing first-seen timestamp")
		}
	}
	return q, nil
}
func materializeCloudQueue(q *CloudQueue, store CampaignStore, now time.Time) (bool, error) {
	desired := map[string]CloudQueueItem{}
	for gigID, c := range store.Campaigns {
		if !durableGigID.MatchString(gigID) {
			return false, errors.New("invalid campaign gig ID")
		}
		for jobID, job := range c.Jobs {
			if job.ID != jobID || job.SourceRevision != c.Revision {
				return false, errors.New("cloud campaign job identity/revision mismatch")
			}
			key := cloudJobKey(gigID, c.Revision, jobID)
			state := cloudDesiredState(job)
			if state == "private-ready" {
				for _, dependencyID := range job.DependsOn {
					dependency, found := c.Jobs[dependencyID]
					if !found || dependency.State != "private-complete" {
						state = "waiting-dependency"
						break
					}
				}
			}
			item := CloudQueueItem{Key: key, GigID: gigID, Revision: c.Revision,
				JobID: jobID, Execution: job.Execution, DesiredState: state,
				FirstSeenAt: now.UTC().Format(time.RFC3339)}
			if prior, exists := q.Items[key]; exists {
				item.FirstSeenAt = prior.FirstSeenAt
			}
			desired[key] = item
		}
	}
	changed := false
	for k, old := range q.Items {
		if _, exists := desired[k]; exists {
			continue
		}
		if old.DesiredState != "superseded" {
			old.DesiredState = "superseded"
			q.Items[k] = old
			changed = true
		}
	}
	for k, item := range desired {
		old, exists := q.Items[k]
		if !exists || old.DesiredState != item.DesiredState || old.Execution != item.Execution {
			q.Items[k] = item
			changed = true
		}
	}
	if len(q.Items) > 10000 {
		return false, errors.New("cloud queue cap exceeded; do not truncate state")
	}
	if changed {
		q.Generation++
		q.UpdatedAt = now.UTC().Format(time.RFC3339)
	}
	return changed, nil
}
func cloudReport(q CloudQueue, count int, changed bool, now time.Time) CloudDryRunReport {
	r := CloudDryRunReport{SchemaVersion: 1, Mode: "DRY_RUN_NO_EXECUTION",
		CheckedAt: now.UTC().Format(time.RFC3339), Campaigns: count,
		QueueItems: len(q.Items), QueueGeneration: q.Generation, QueueChanged: changed}
	for _, item := range q.Items {
		switch item.DesiredState {
		case "private-ready":
			r.PrivateReady++
		case "human-handoff":
			r.HumanHandoffs++
		case "superseded":
			r.Superseded++
		default:
			r.Blocked++
		}
	}
	return r
}

// RunCloudDryRun is a single bounded iteration. systemd.timer handles
// scheduling and boot catch-up; this code never starts a network listener,
// provider connector, agent, background loop, or public posting operation.
func RunCloudDryRun(dir string, now time.Time) (CloudDryRunReport, error) {
	if err := validCloudStateDir(dir); err != nil {
		return CloudDryRunReport{}, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return CloudDryRunReport{}, err
	}
	lockPath := filepath.Join(dir, ".cloud-worker.lock")
	if err := privatePath(lockPath); err != nil {
		return CloudDryRunReport{}, err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return CloudDryRunReport{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return CloudDryRunReport{}, fmt.Errorf("worker already running; refuse overlap: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	storePath := filepath.Join(dir, "campaigns.json")
	store, err := LoadCampaignStore(storePath)
	if err != nil {
		return CloudDryRunReport{}, err
	}
	queuePath := filepath.Join(dir, "queue.json")
	queue, err := loadCloudQueue(queuePath)
	if err != nil {
		return CloudDryRunReport{}, err
	}
	changed, err := materializeCloudQueue(&queue, store, now)
	if err != nil {
		return CloudDryRunReport{}, err
	}
	if queue.Generation == 0 {
		queue.Generation = 1
		queue.UpdatedAt = now.UTC().Format(time.RFC3339)
		changed = true
	}
	if changed {
		if err := savePrivateJSON(queuePath, queue); err != nil {
			return CloudDryRunReport{}, err
		}
	}
	report := cloudReport(queue, len(store.Campaigns), changed, now)
	if err := savePrivateJSON(filepath.Join(dir, "heartbeat.json"), report); err != nil {
		return CloudDryRunReport{}, err
	}
	return report, nil
}

// CheckCloudDryRunHealth only checks persistent state; it cannot assert
// external uptime or actual job execution. Stale/invalid heartbeat fails.
func CheckCloudDryRunHealth(dir string, now time.Time, maxAge time.Duration) (CloudDryRunReport, error) {
	if err := validCloudStateDir(dir); err != nil {
		return CloudDryRunReport{}, err
	}
	if maxAge <= 0 {
		return CloudDryRunReport{}, errors.New("maxAge must be positive")
	}
	var report CloudDryRunReport
	if err := readPrivateJSON(filepath.Join(dir, "heartbeat.json"), &report); err != nil {
		return CloudDryRunReport{}, err
	}
	if report.SchemaVersion != 1 || report.Mode != "DRY_RUN_NO_EXECUTION" ||
		report.ProviderCalls != 0 || report.PublicWrites != 0 || report.ExecutedJobs != 0 {
		return CloudDryRunReport{}, errors.New("unsafe or invalid cloud worker heartbeat")
	}
	t, err := time.Parse(time.RFC3339, report.CheckedAt)
	if err != nil || now.Sub(t) > maxAge || t.After(now.Add(30*time.Second)) {
		return CloudDryRunReport{}, errors.New("dry-run heartbeat is stale or future-dated")
	}
	return report, nil
}

// CloudQueueSnapshot returns IDs only; never raw event media or credentials.
func CloudQueueSnapshot(dir string) ([]CloudQueueItem, error) {
	if err := validCloudStateDir(dir); err != nil {
		return nil, err
	}
	q, err := loadCloudQueue(filepath.Join(dir, "queue.json"))
	if err != nil {
		return nil, err
	}
	items := make([]CloudQueueItem, 0, len(q.Items))
	for _, item := range q.Items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items, nil
}
