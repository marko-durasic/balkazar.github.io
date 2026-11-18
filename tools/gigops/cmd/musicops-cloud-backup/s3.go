package main

// Versioned S3 snapshot adapter. It reuses DuReef's already-defined private
// CAO state bucket and requires S3-managed encryption; no new bucket, KMS key,
// Advanced SSM tier, paid service or credential provider is created here.
import (
	gigops "balkazar.org/musicops"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const fixedS3BackupKey = "musicops/dryrun/cloud-state.json"

var safeS3VersionID = regexp.MustCompile("^[A-Za-z0-9_.+/-]{1,256}$")
var s3BucketNameRE = regexp.MustCompile("^dureef-cao-state-[0-9]{12}$")

type awsS3 struct {
	cli    awsSSM
	bucket string
}

func (s awsS3) validate(ctx context.Context) error {
	if !s3BucketNameRE.MatchString(s.bucket) {
		return errors.New("refusing S3 bucket outside deterministic DuReef CAO backup namespace")
	}
	// Verify the target bucket belongs to this exact account, not merely a
	// lookalike bucket that matches a global naming pattern.
	who, err := s.cli.invoke(ctx, []string{"sts", "get-caller-identity", "--query", "Account", "--output", "text"}, nil)
	if err != nil {
		return err
	}
	account := strings.TrimSpace(string(who))
	if len(account) != 12 || s.bucket != "dureef-cao-state-"+account {
		return errors.New("S3 backup bucket is not in the authenticated AWS account")
	}
	// Required because the object key is stable and we rely on version history.
	raw, err := s.cli.invoke(ctx, []string{
		"s3api", "get-bucket-versioning", "--bucket", s.bucket,
		"--region", fixedBackupRegion, "--output", "json",
	}, nil)
	if err != nil {
		return fmt.Errorf("cannot verify versioning on existing S3 bucket: %w", err)
	}
	var version struct {
		Status string `json:"Status"`
	}
	if err := json.Unmarshal(raw, &version); err != nil {
		return err
	}
	if version.Status != "Enabled" {
		return errors.New("existing S3 bucket versioning not enabled; refusing backup overwrite")
	}
	return nil
}
func (s awsS3) upload(ctx context.Context, encoded string) (string, error) {
	if _, err := gigops.DecodeCloudObjectBackup(encoded); err != nil {
		return "", err
	}
	if err := s.validate(ctx); err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", ".musicops-s3-backup-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return "", err
	}
	if _, err := file.WriteString(encoded); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	output, err := s.cli.invoke(ctx, []string{
		"s3api", "put-object", "--bucket", s.bucket, "--key", fixedS3BackupKey,
		"--body", file.Name(), "--server-side-encryption", "AES256",
		"--content-type", "application/json", "--region", fixedBackupRegion, "--output", "json",
	}, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		VersionID string `json:"VersionId"`
		SSE       string `json:"ServerSideEncryption"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", err
	}
	if response.VersionID == "" || response.VersionID == "null" || !safeS3VersionID.MatchString(response.VersionID) ||
		response.SSE != "AES256" {
		return "", errors.New("S3 did not confirm a versioned SSE-S3 object; stop and inspect")
	}
	return response.VersionID, nil
}
func (s awsS3) fetch(ctx context.Context, version string) (string, error) {
	if !safeS3VersionID.MatchString(version) || version == "null" {
		return "", errors.New("exact S3 object version ID required")
	}
	if err := s.validate(ctx); err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", ".musicops-s3-readback-*.json")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	defer os.Remove(path)
	responseBytes, err := s.cli.invoke(ctx, []string{
		"s3api", "get-object", "--bucket", s.bucket, "--key", fixedS3BackupKey,
		"--version-id", version, "--region", fixedBackupRegion,
		"--output", "json", path,
	}, nil)
	if err != nil {
		return "", err
	}
	var response struct {
		VersionID string `json:"VersionId"`
		SSE       string `json:"ServerSideEncryption"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return "", err
	}
	if response.VersionID != version || response.SSE != "AES256" {
		return "", errors.New("S3 readback version/encryption mismatch")
	}
	stat, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if stat.Size() > maxS3BackupObjectBytes || !stat.Mode().IsRegular() || stat.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("S3 object is oversized or not a regular file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if _, err := gigops.DecodeCloudObjectBackup(string(body)); err != nil {
		return "", err
	}
	return string(body), nil
}

const maxS3BackupObjectBytes = 256 << 10
