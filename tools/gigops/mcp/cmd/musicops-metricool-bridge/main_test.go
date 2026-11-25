package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gigops "balkazar.org/musicops"
)

func testProtectedJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func testOperatorConfig(t *testing.T) (string, operatorConfig) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	inbox := filepath.Join(root, "inbox")
	if err := os.MkdirAll(inbox, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := operatorConfig{
		Version:              1,
		CampaignStore:        filepath.Join(root, "campaigns.json"),
		OwnerSignedGrant:     filepath.Join(root, "signed.json"),
		OwnerPinnedPublicKey: filepath.Join(root, "public.key"),
		OwnerRevocations:     filepath.Join(root, "revocations.json"),
		ApprovedInbox:        inbox,
		OAuthProxyExecutable: filepath.Join(root, "metricool-proxy"),
	}
	path := filepath.Join(root, "bridge.json")
	testProtectedJSON(t, path, cfg)
	return path, cfg
}
func TestProtectedOperatorConfigAndCandidateRouting(t *testing.T) {
	path, cfg := testOperatorConfig(t)
	got, err := readOperatorConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.CampaignStore != cfg.CampaignStore {
		t.Fatal("wrong config path")
	}
	candidate := gigops.MetricoolDraftCandidate{GigID: "balkazar-dec-show", SourceRevision: 1, Text: "Draft only"}
	file := filepath.Join(cfg.ApprovedInbox, "candidate.json")
	testProtectedJSON(t, file, candidate)
	loaded, err := readCandidate(cfg, file)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GigID != candidate.GigID {
		t.Fatal("candidate read failed")
	}
	other := filepath.Join(filepath.Dir(path), "outside.json")
	testProtectedJSON(t, other, candidate)
	if _, err := readCandidate(cfg, other); err == nil {
		t.Fatal("accepted candidate from outside approved inbox")
	}
	link := filepath.Join(cfg.ApprovedInbox, "link.json")
	if err := os.Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readCandidate(cfg, link); err == nil {
		t.Fatal("accepted symlink candidate")
	}
}
func TestCannotSubstituteUntrustedOperatorConfig(t *testing.T) {
	path, cfg := testOperatorConfig(t)
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperatorConfig(path); err == nil {
		t.Fatal("accepted world-writable config")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(path)
	if err := os.Chmod(parent, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperatorConfig(path); err == nil {
		t.Fatal("accepted writable config parent")
	}
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(cfg.ApprovedInbox, "bridge-link.json")
	if err := os.Symlink(path, copyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperatorConfig(copyPath); err == nil {
		t.Fatal("accepted symlink config")
	}
}
func TestOwnerPinnedTrustAndRevocationReadback(t *testing.T) {
	_, cfg := testOperatorConfig(t)
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	now := time.Now()
	g := gigops.MetricoolDraftGrant{
		Version: 1, GrantID: "owner-draft-contract", Artist: "Balkazar", BrandID: "7273414", GigID: "balkazar-dec-show",
		Action: "create-private-draft", Networks: []string{"facebook"},
		MediaURLAllowlist: []string{"https://balkazar.org/assets/images/gallery/g-christmas-market-horns.jpg"},
		ValidFrom:         now.Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt:         now.Add(time.Hour).Format(time.RFC3339),
		MaxDrafts:         1, SpendLimitNTD: 0,
	}
	signed, err := gigops.SignOwnerDraftGrant(g, private, now)
	if err != nil {
		t.Fatal(err)
	}
	testProtectedJSON(t, cfg.OwnerSignedGrant, signed)
	pub := base64.RawStdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	if err := os.WriteFile(cfg.OwnerPinnedPublicKey, []byte(pub+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	testProtectedJSON(t, cfg.OwnerRevocations, []string{})
	read, trust, err := readTrust(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gigops.VerifyMetricoolDraftGrant(read, trust, now); err != nil {
		t.Fatal(err)
	}
	testProtectedJSON(t, cfg.OwnerRevocations, []string{g.GrantID})
	_, revokedTrust, err := readTrust(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gigops.VerifyMetricoolDraftGrant(read, revokedTrust, now); err == nil {
		t.Fatal("revocation ignored in trusted runner")
	}
	if err := os.Chmod(cfg.OwnerPinnedPublicKey, 0664); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readTrust(cfg); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatal("accepted unprotected owner identity file")
	}
}
