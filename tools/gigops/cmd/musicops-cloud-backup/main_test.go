package main

import (
	gigops "balkazar.org/musicops"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testSSMBackup(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private-state")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store := gigops.NewCampaignStore()
	if err := gigops.SaveCampaignStore(filepath.Join(dir, "campaigns.json"), store); err != nil {
		t.Fatal(err)
	}
	when := time.Now()
	if _, err := gigops.RunCloudDryRun(dir, when); err != nil {
		t.Fatal(err)
	}
	blob, _, err := gigops.PrepareCloudBackup(dir, when)
	if err != nil {
		t.Fatal(err)
	}
	return blob
}
func testFakeAWS(t *testing.T, tier string) (awsSSM, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-aws")
	inputFile := filepath.Join(t.TempDir(), "input.json")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = 'ssm' ] && [ \"$2\" = 'put-parameter' ]; then\n" +
		" for arg in \"$@\"; do case \"$arg\" in file://*) cat \"${arg#file://}\" > \"$MOCK_AWS_INPUT\";; esac; done\n" +
		" printf '%s\\n' '{\"Version\":7,\"Tier\":\"" + tier + "\"}'\n" +
		"elif [ \"$1\" = 'ssm' ] && [ \"$2\" = 'get-parameter' ]; then\n" +
		" printf '%s\\n' \"$MOCK_AWS_GET\"\n" +
		"else\n exit 10\nfi\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCK_AWS_INPUT", inputFile)
	return awsSSM{bin: path}, inputFile
}
func TestAWSStandardSecureStringIsExplicitAndVersionReadback(t *testing.T) {
	blob := testSSMBackup(t)
	client, inputFile := testFakeAWS(t, "Standard")
	v, err := client.upload(context.Background(), blob)
	if err != nil || v != 7 {
		t.Fatalf("standard backup failed: %v version %d", err, v)
	}
	raw, err := os.ReadFile(inputFile)
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Name      string `json:"Name"`
		Value     string `json:"Value"`
		Type      string `json:"Type"`
		Tier      string `json:"Tier"`
		Overwrite bool   `json:"Overwrite"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	if input.Name != fixedBackupName || input.Type != "SecureString" || input.Tier != "Standard" || !input.Overwrite || input.Value != blob {
		t.Fatalf("unsafe AWS PutParameter parameters: %+v", input)
	}
	response, _ := json.Marshal(map[string]any{"Parameter": map[string]any{"Type": "SecureString", "Version": 7, "Value": blob}})
	t.Setenv("MOCK_AWS_GET", string(response))
	got, err := client.fetch(context.Background(), 7)
	if err != nil || got != blob {
		t.Fatalf("version-pinned readback mismatch: %v", err)
	}
}
func TestAWSFailsClosedOnUnexpectedAdvancedTierOrBadVersion(t *testing.T) {
	blob := testSSMBackup(t)
	client, _ := testFakeAWS(t, "Advanced")
	if _, err := client.upload(context.Background(), blob); err == nil {
		t.Fatal("accepted Advanced tier")
	}
	if _, err := client.upload(context.Background(), strings.Repeat("x", 4001)); err == nil {
		t.Fatal("accepted oversize backup")
	}
	if _, err := client.fetch(context.Background(), 0); err == nil {
		t.Fatal("accepted unpinned version")
	}
}
