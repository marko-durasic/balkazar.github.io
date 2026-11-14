package main

// Off-host AWS Systems Manager Standard SecureString adapter. No paid
// Advanced parameter, no higher-throughput setting changes, no new bucket
// or compute. OAuth/signing keys are NEVER included in the snapshot.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	gigops "balkazar.org/musicops"
)

const fixedBackupName = "/dureef/musicops/cloud-dryrun-backup"
const fixedBackupRegion = "us-east-1"
const maxCLIResult = 6000

type awsSSM struct {
	bin string
}
type ssmVersion struct {
	Version int    `json:"Version"`
	Tier    string `json:"Tier,omitempty"`
}

func (s awsSSM) invoke(ctx context.Context, args []string, stdin []byte) ([]byte, error) {
	if s.bin == "" {
		return nil, errors.New("AWS CLI executable not configured")
	}
	// AWS CLI v2 does not reliably consume file:///dev/stdin for this
	// operation. Use a protected ephemeral file rather than putting the
	// encrypted snapshot payload in process arguments or shell history.
	if stdin != nil {
		file, err := os.CreateTemp("", ".musicops-ssm-input-*.json")
		if err != nil {
			return nil, err
		}
		defer os.Remove(file.Name())
		if err := file.Chmod(0600); err != nil {
			file.Close()
			return nil, err
		}
		if _, err := file.Write(stdin); err != nil {
			file.Close()
			return nil, err
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		for i, arg := range args {
			if arg == "file:///dev/stdin" {
				args[i] = "file://" + file.Name()
			}
		}
	}
	cmd := exec.CommandContext(ctx, s.bin, args...)
	cmd.Env = os.Environ()
	// Response is suppressed from stdout and only parsed by this process.
	// It must never contain an arbitrary parameter value in diagnostic logs.
	var output bytes.Buffer
	var errs bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &errs
	if err := cmd.Run(); err != nil {
		code := "unknown"
		for _, known := range []string{"AccessDeniedException", "AccessDenied", "ParameterAlreadyExists",
			"ValidationException", "ParamValidation", "InvalidParameters", "ParameterNotFound",
			"InvalidKeyId", "KMSAccessDeniedException"} {
			if strings.Contains(errs.String(), known) {
				code = known
				break
			}
		}
		return nil, fmt.Errorf("AWS SSM operation failed (sanitized class %s; no parameter data logged): %w", code, err)
	}
	if output.Len() > maxCLIResult {
		return nil, errors.New("AWS SSM response too large")
	}
	return output.Bytes(), nil
}

func (s awsSSM) upload(ctx context.Context, encoded string) (int, error) {
	if len(encoded) == 0 || len(encoded) > 4000 {
		return 0, errors.New("backup exceeds free Standard Parameter Store limit")
	}
	if _, err := gigops.DecodeCloudBackup(encoded); err != nil {
		return 0, err
	}
	input := map[string]any{
		"Name":        fixedBackupName,
		"Value":       encoded,
		"Type":        "SecureString",
		"Tier":        "Standard",
		"Overwrite":   true,
		"Description": "Balkazar MusicOps private dry-run queue backup, versioned Standard SecureString (no OAuth or signing keys)",
	}
	data, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	result, err := s.invoke(ctx, []string{"ssm", "put-parameter", "--region", fixedBackupRegion,
		"--cli-input-json", "file:///dev/stdin", "--output", "json"}, data)
	if err != nil {
		return 0, err
	}
	var v ssmVersion
	if err := json.Unmarshal(result, &v); err != nil {
		return 0, err
	}
	if v.Version < 1 || v.Tier != "Standard" {
		return 0, errors.New("AWS did not confirm free Standard tier; stop backup uploads and inspect billing")
	}
	return v.Version, nil
}
func (s awsSSM) fetch(ctx context.Context, version int) (string, error) {
	if version < 1 {
		return "", errors.New("exact SSM backup version required")
	}
	name := fixedBackupName + ":" + strconv.Itoa(version)
	output, err := s.invoke(ctx, []string{"ssm", "get-parameter", "--region", fixedBackupRegion,
		"--name", name, "--with-decryption", "--output", "json"}, nil)
	if err != nil {
		return "", err
	}
	var p struct {
		Parameter struct {
			Value   string `json:"Value"`
			Type    string `json:"Type"`
			Version int    `json:"Version"`
		} `json:"Parameter"`
	}
	if err := json.Unmarshal(output, &p); err != nil {
		return "", err
	}
	if p.Parameter.Type != "SecureString" || p.Parameter.Version != version {
		return "", errors.New("readback was not the requested encrypted backup version")
	}
	if _, err := gigops.DecodeCloudBackup(p.Parameter.Value); err != nil {
		return "", err
	}
	return p.Parameter.Value, nil
}

type receipt struct {
	Mode          string `json:"mode"`
	SSMParameter  string `json:"ssm_parameter"`
	Version       int    `json:"version,omitempty"`
	EncodedBytes  int    `json:"encoded_bytes"`
	BackupSHA256  string `json:"backup_sha256"`
	RestoredTo    string `json:"restored_to,omitempty"`
	Campaigns     int    `json:"campaigns"`
	QueueItems    int    `json:"queue_items"`
	CloudWrites   int    `json:"cloud_writes"`
	ProviderCalls int    `json:"provider_calls"`
	PublicWrites  int    `json:"public_writes"`
}

func run() error {
	mode := flag.String("mode", "preview", "preview, upload-standard, restore-version")
	dir := flag.String("state-dir", "", "Absolute private MusicOps cloud state directory")
	version := flag.Int("version", 0, "Specific version of the remote backup for restore-version")
	target := flag.String("restore-dir", "", "Fresh empty path for checked restore; never overwrite production")
	confirm := flag.Bool("allow-standard-securestring", false, "Explicitly permit one standard-tier encrypted backup write/read")
	flag.Parse()
	if *dir == "" {
		return errors.New("--state-dir required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	aws := awsSSM{bin: "/usr/local/bin/aws"}
	var out receipt
	switch *mode {
	case "preview":
		_, p, err := gigops.PrepareCloudBackup(*dir, time.Now())
		if err != nil {
			return err
		}
		out = receipt{Mode: "OFFHOST_BACKUP_PREVIEW_ONLY", EncodedBytes: p.EncodedBytes,
			BackupSHA256: p.SnapshotSHA256, Campaigns: p.Campaigns, QueueItems: p.QueueItems}
	case "upload-standard":
		if !*confirm {
			return errors.New("refusing AWS write without explicit --allow-standard-securestring")
		}
		encoded, p, err := gigops.PrepareCloudBackup(*dir, time.Now())
		if err != nil {
			return err
		}
		v, err := aws.upload(ctx, encoded)
		if err != nil {
			return err
		}
		readback, err := aws.fetch(ctx, v)
		if err != nil {
			return fmt.Errorf("backup version %d created but readback failed: %w", v, err)
		}
		if readback != encoded {
			return errors.New("AWS backup version content mismatch; do not claim success")
		}
		out = receipt{Mode: "OFFHOST_STANDARD_SECURESTRING_VERIFIED", SSMParameter: fixedBackupName, Version: v,
			EncodedBytes: p.EncodedBytes, BackupSHA256: p.SnapshotSHA256,
			Campaigns: p.Campaigns, QueueItems: p.QueueItems, CloudWrites: 1}
	case "restore-version":
		if !*confirm {
			return errors.New("refusing AWS decryption without explicit --allow-standard-securestring")
		}
		if *target == "" || strings.TrimSpace(*target) == strings.TrimSpace(*dir) {
			return errors.New("--restore-dir must be a distinct fresh path")
		}
		encrypted, err := aws.fetch(ctx, *version)
		if err != nil {
			return err
		}
		p, err := gigops.RestoreCloudBackup(encrypted, *target, time.Now())
		if err != nil {
			return err
		}
		out = receipt{Mode: "RESTORED_FROM_REMOTE_VERSION_TO_NEW_DIRECTORY", SSMParameter: fixedBackupName,
			Version: *version, EncodedBytes: p.EncodedBytes, BackupSHA256: p.SnapshotSHA256,
			RestoredTo: *target, Campaigns: p.Campaigns, QueueItems: p.QueueItems}
	default:
		return errors.New("unsupported backup mode")
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "MusicOps backup:", err)
		os.Exit(2)
	}
}
