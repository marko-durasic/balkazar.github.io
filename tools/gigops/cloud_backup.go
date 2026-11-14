package gigops

// Cloud snapshot encoder/restore: stdlib only and NO cloud network calls.
// A separate authenticated adapter may transmit the compact encrypted backup
// to AWS SSM Standard SecureString, but never to a paid Advanced parameter.
import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxCloudBackupBytes = 4000
const maxCloudBackupUnpacked = 2 << 20
const cloudBackupSchemaVersion = 1

var cloudBackupFiles = []string{"campaigns.json", "queue.json", "heartbeat.json"}

type CloudBackupContents struct {
	SchemaVersion int                        `json:"schema_version"`
	CreatedAt     string                     `json:"created_at"`
	Files         map[string]json.RawMessage `json:"files"`
	FileSHA256    map[string]string          `json:"file_sha256"`
}
type CloudBackupEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	SHA256        string `json:"sha256"`
	GZIPBase64    string `json:"gzip_base64"`
}
type CloudBackupReport struct {
	SchemaVersion  int    `json:"schema_version"`
	CreatedAt      string `json:"created_at"`
	EncodedBytes   int    `json:"encoded_bytes"`
	SnapshotSHA256 string `json:"snapshot_sha256"`
	Campaigns      int    `json:"campaigns"`
	QueueItems     int    `json:"queue_items"`
	Mode           string `json:"mode"`
	PublicWrites   int    `json:"public_writes"`
	ProviderCalls  int    `json:"provider_calls"`
}

// Validate the actual local state before treating a copy as restorable.
func validateCloudBackupDirectory(dir string, now time.Time) (int, int, error) {
	if err := validCloudStateDir(dir); err != nil {
		return 0, 0, err
	}
	store, err := LoadCampaignStore(filepath.Join(dir, "campaigns.json"))
	if err != nil {
		return 0, 0, err
	}
	queue, err := loadCloudQueue(filepath.Join(dir, "queue.json"))
	if err != nil {
		return 0, 0, err
	}
	_, err = CheckCloudDryRunHealth(dir, now, 30*time.Minute)
	if err != nil {
		return 0, 0, err
	}
	// A backup taken while a gig update has not yet reached the queue must not
	// silently count as a consistent restored campaign.
	expected := emptyCloudQueue()
	_, err = materializeCloudQueue(&expected, store, now)
	if err != nil {
		return 0, 0, err
	}
	for key, item := range expected.Items {
		present, ok := queue.Items[key]
		if !ok || present.DesiredState != item.DesiredState {
			return 0, 0, errors.New("cloud queue does not match current campaign revision; wait for worker")
		}
	}
	return len(store.Campaigns), len(queue.Items), nil
}

func PrepareCloudBackup(dir string, now time.Time) (string, CloudBackupReport, error) {
	campaigns, items, err := validateCloudBackupDirectory(dir, now)
	if err != nil {
		return "", CloudBackupReport{}, err
	}
	snap := CloudBackupContents{
		SchemaVersion: cloudBackupSchemaVersion, CreatedAt: now.UTC().Format(time.RFC3339),
		Files: make(map[string]json.RawMessage), FileSHA256: make(map[string]string),
	}
	for _, name := range cloudBackupFiles {
		path := filepath.Join(dir, name)
		if err := privatePath(path); err != nil {
			return "", CloudBackupReport{}, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", CloudBackupReport{}, err
		}
		if !json.Valid(data) {
			return "", CloudBackupReport{}, fmt.Errorf("invalid JSON state file %s", name)
		}
		var canonical bytes.Buffer
		if err := json.Compact(&canonical, data); err != nil {
			return "", CloudBackupReport{}, err
		}
		snap.Files[name] = json.RawMessage(canonical.Bytes())
		sha := sha256.Sum256(canonical.Bytes())
		snap.FileSHA256[name] = hex.EncodeToString(sha[:])
	}
	plain, err := json.Marshal(snap)
	if err != nil {
		return "", CloudBackupReport{}, err
	}
	if len(plain) > maxCloudBackupUnpacked {
		return "", CloudBackupReport{}, errors.New("cloud state too large for backup safety limit")
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(plain); err != nil {
		return "", CloudBackupReport{}, err
	}
	if err := writer.Close(); err != nil {
		return "", CloudBackupReport{}, err
	}
	sha := sha256.Sum256(plain)
	env := CloudBackupEnvelope{
		SchemaVersion: cloudBackupSchemaVersion, SHA256: hex.EncodeToString(sha[:]),
		GZIPBase64: base64.StdEncoding.EncodeToString(compressed.Bytes()),
	}
	serialized, err := json.Marshal(env)
	if err != nil {
		return "", CloudBackupReport{}, err
	}
	if len(serialized) > maxCloudBackupBytes {
		return "", CloudBackupReport{}, fmt.Errorf("compressed backup %d bytes exceeds Standard Parameter Store limit %d; never upgrade to paid Advanced automatically", len(serialized), maxCloudBackupBytes)
	}
	return string(serialized), CloudBackupReport{
		SchemaVersion: cloudBackupSchemaVersion, CreatedAt: snap.CreatedAt, EncodedBytes: len(serialized),
		SnapshotSHA256: env.SHA256, Campaigns: campaigns, QueueItems: items,
		Mode: "PREPARED_OFFHOST_BACKUP_NOT_UPLOADED", PublicWrites: 0, ProviderCalls: 0,
	}, nil
}

func DecodeCloudBackup(encoded string) (CloudBackupContents, error) {
	if len(encoded) > maxCloudBackupBytes || len(encoded) == 0 {
		return CloudBackupContents{}, errors.New("backup exceeds Standard Parameter Store limit")
	}
	var env CloudBackupEnvelope
	dec := json.NewDecoder(bytes.NewBufferString(encoded))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return CloudBackupContents{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return CloudBackupContents{}, errors.New("backup has trailing data")
	}
	if env.SchemaVersion != cloudBackupSchemaVersion || !storedFingerprintRE.MatchString(env.SHA256) {
		return CloudBackupContents{}, errors.New("unsupported backup envelope or digest")
	}
	zipped, err := base64.StdEncoding.DecodeString(env.GZIPBase64)
	if err != nil {
		return CloudBackupContents{}, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return CloudBackupContents{}, err
	}
	defer gz.Close()
	plain, err := io.ReadAll(io.LimitReader(gz, maxCloudBackupUnpacked+1))
	if err != nil || len(plain) > maxCloudBackupUnpacked {
		return CloudBackupContents{}, errors.New("corrupt or oversized backup decompression")
	}
	sha := sha256.Sum256(plain)
	if hex.EncodeToString(sha[:]) != env.SHA256 {
		return CloudBackupContents{}, errors.New("backup SHA256 mismatch")
	}
	var data CloudBackupContents
	dec = json.NewDecoder(bytes.NewReader(plain))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&data); err != nil {
		return CloudBackupContents{}, err
	}
	if data.SchemaVersion != cloudBackupSchemaVersion || len(data.Files) != len(cloudBackupFiles) || len(data.FileSHA256) != len(cloudBackupFiles) {
		return CloudBackupContents{}, errors.New("wrong backup contents schema/files")
	}
	if _, err := time.Parse(time.RFC3339, data.CreatedAt); err != nil {
		return CloudBackupContents{}, err
	}
	for _, name := range cloudBackupFiles {
		payload, ok := data.Files[name]
		if !ok || !json.Valid(payload) {
			return CloudBackupContents{}, fmt.Errorf("backup missing valid %s", name)
		}
		s := sha256.Sum256(payload)
		if data.FileSHA256[name] != hex.EncodeToString(s[:]) {
			return CloudBackupContents{}, fmt.Errorf("backup file digest mismatch %s", name)
		}
	}
	return data, nil
}

// RestoreCloudBackup stages into a fresh PRIVATE directory, never overwrites
// production. It validates all 3 files and dry-run queue consistency before
// reporting restorable. No provider call, public network, or credentials.
func RestoreCloudBackup(encoded, target string, now time.Time) (CloudBackupReport, error) {
	data, err := DecodeCloudBackup(encoded)
	if err != nil {
		return CloudBackupReport{}, err
	}
	if err := validCloudStateDir(target); err != nil {
		return CloudBackupReport{}, err
	}
	if _, err := os.Lstat(target); err == nil {
		return CloudBackupReport{}, errors.New("restore destination must not already exist; refusing production overwrite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return CloudBackupReport{}, err
	}
	if err := os.Mkdir(target, 0700); err != nil {
		return CloudBackupReport{}, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(target)
		}
	}()
	for _, name := range cloudBackupFiles {
		if err := savePrivateJSONRaw(filepath.Join(target, name), data.Files[name]); err != nil {
			return CloudBackupReport{}, err
		}
	}
	// Check schema and queue consistency without requiring historical heartbeat
	// to still be within 30 minutes (backups may be months old).
	store, err := LoadCampaignStore(filepath.Join(target, "campaigns.json"))
	if err != nil {
		return CloudBackupReport{}, err
	}
	queue, err := loadCloudQueue(filepath.Join(target, "queue.json"))
	if err != nil {
		return CloudBackupReport{}, err
	}
	var heartbeat CloudDryRunReport
	if err := readPrivateJSON(filepath.Join(target, "heartbeat.json"), &heartbeat); err != nil {
		return CloudBackupReport{}, err
	}
	if heartbeat.Mode != "DRY_RUN_NO_EXECUTION" || heartbeat.PublicWrites != 0 || heartbeat.ProviderCalls != 0 || heartbeat.ExecutedJobs != 0 {
		return CloudBackupReport{}, errors.New("backup heartbeat reports unsafe actions")
	}
	for gigID, c := range store.Campaigns {
		for jobID, job := range c.Jobs {
			k := cloudJobKey(gigID, c.Revision, jobID)
			observed, ok := queue.Items[k]
			if !ok || observed.DesiredState != cloudDesiredStateWithDeps(c, job) {
				return CloudBackupReport{}, errors.New("restored queue and campaign jobs inconsistent")
			}
		}
	}
	success = true
	return CloudBackupReport{SchemaVersion: cloudBackupSchemaVersion, CreatedAt: data.CreatedAt, EncodedBytes: len(encoded),
		SnapshotSHA256: func() string { b, _ := json.Marshal(data); s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }(),
		Campaigns:      len(store.Campaigns), QueueItems: len(queue.Items),
		Mode: "RESTORED_TO_NEW_PRIVATE_DIRECTORY", PublicWrites: 0, ProviderCalls: 0}, nil
}

func cloudDesiredStateWithDeps(c StoredCampaign, job CampaignJobRecord) string {
	state := cloudDesiredState(job)
	if state == "private-ready" {
		for _, id := range job.DependsOn {
			dep, ok := c.Jobs[id]
			if !ok || dep.State != "private-complete" {
				return "waiting-dependency"
			}
		}
	}
	return state
}
func savePrivateJSONRaw(path string, raw json.RawMessage) error {
	if !json.Valid(raw) {
		return errors.New("invalid raw JSON for restored state")
	}
	dir := filepath.Dir(path)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	folder, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer folder.Close()
	return folder.Sync()
}
