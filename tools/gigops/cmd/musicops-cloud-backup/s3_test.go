package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeS3CLI(t *testing.T, versioning string) (awsS3, string) {
	t.Helper()
	root := t.TempDir()
	exe := filepath.Join(root, "aws-mock")
	bodyPath := filepath.Join(root, "remote-object")
	logPath := filepath.Join(root, "calls")
	script := `#!/bin/sh
printf '%s
' "$*" >> "$MOCK_S3_LOG"
if [ "$1" = "sts" ] && [ "$2" = "get-caller-identity" ]; then
 printf '541144661723
'
elif [ "$1" = "s3api" ] && [ "$2" = "get-bucket-versioning" ]; then
 printf '{"Status":"%s"}
' "$MOCK_S3_VERSIONING"
elif [ "$1" = "s3api" ] && [ "$2" = "put-object" ]; then
 while [ "$#" -gt 0 ]; do
  if [ "$1" = "--body" ]; then shift; cp "$1" "$MOCK_S3_OBJECT"; break; fi
  shift
 done
 printf '{"VersionId":"V1_test_123","ServerSideEncryption":"AES256"}
'
elif [ "$1" = "s3api" ] && [ "$2" = "get-object" ]; then
 for last; do :; done
 cp "$MOCK_S3_OBJECT" "$last"
 printf '{"VersionId":"V1_test_123","ServerSideEncryption":"AES256"}
'
else
 exit 17
fi
`
	if err := os.WriteFile(exe, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCK_S3_LOG", logPath)
	t.Setenv("MOCK_S3_OBJECT", bodyPath)
	t.Setenv("MOCK_S3_VERSIONING", versioning)
	return awsS3{cli: awsSSM{bin: exe}, bucket: "dureef-cao-state-541144661723"}, logPath
}

func TestS3ExistingVersionedEncryptedBucketRoundTrip(t *testing.T) {
	client, logPath := fakeS3CLI(t, "Enabled")
	blob := testSSMBackup(t)
	version, err := client.upload(context.Background(), blob)
	if err != nil || version != "V1_test_123" {
		t.Fatalf("upload failed: %q %v", version, err)
	}
	got, err := client.fetch(context.Background(), version)
	if err != nil || got != blob {
		t.Fatalf("versioned readback failed: %v", err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	for _, required := range []string{"get-caller-identity", "get-bucket-versioning", "put-object", "get-object",
		"--server-side-encryption AES256", "--version-id V1_test_123", "--key musicops/dryrun/cloud-state.json"} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing safety control %s", required)
		}
	}
	if strings.Contains(text, "delete-object") || strings.Contains(text, "create-bucket") {
		t.Fatal("S3 adapter attempted destructive or paid infrastructure operation")
	}
}
func TestS3BlocksUnknownOrUnversionedDestination(t *testing.T) {
	client, logPath := fakeS3CLI(t, "Suspended")
	blob := testSSMBackup(t)
	if _, err := client.upload(context.Background(), blob); err == nil {
		t.Fatal("accepted unversioned bucket")
	}
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "put-object") {
		t.Fatal("wrote before versioning check")
	}
	client.bucket = "attacker-owned-bucket"
	if _, err := client.upload(context.Background(), blob); err == nil {
		t.Fatal("accepted unrelated bucket")
	}
	client.bucket = "dureef-cao-state-999999999999"
	if _, err := client.upload(context.Background(), blob); err == nil {
		t.Fatal("accepted wrong account bucket")
	}
}
func TestS3RefusesAmbiguousVersion(t *testing.T) {
	client, _ := fakeS3CLI(t, "Enabled")
	if _, err := client.fetch(context.Background(), ""); err == nil {
		t.Fatal("accepted unpinned version")
	}
	if _, err := client.fetch(context.Background(), "../bad spaces"); err == nil {
		t.Fatal("accepted invalid version token")
	}
}
