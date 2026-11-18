package gigops

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOwnerOnlyKeyGenerationAndPrivateStorage(t *testing.T) {
	public, private, err := GenerateOwnerKeypair()
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != ed25519.PublicKeySize || len(private) != ed25519.PrivateKeySize {
		t.Fatal("invalid generated keypair")
	}
	path := filepath.Join(t.TempDir(), "owner", "private.key")
	if err := WriteOwnerKeyExclusive(path, private); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("owner seed not private: %v", fileInfo.Mode().Perm())
	}
	got, err := ReadOwnerPrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(private) {
		t.Fatal("seed did not round-trip")
	}
	if err := WriteOwnerKeyExclusive(path, private); err == nil {
		t.Fatal("overwrote existing signing key")
	}
}
func TestOwnerGrantSignerCannotWidenOrInventApproval(t *testing.T) {
	_, priv := ownerTestKey()
	g := approvedGrant(sampleGig().GigID)
	signed, err := SignOwnerDraftGrant(g, priv, draftNow())
	if err != nil {
		t.Fatal(err)
	}
	trust := MetricoolGrantTrust{OwnerPublicKey: priv.Public().(ed25519.PublicKey), RevokedGrantIDs: map[string]bool{}}
	if _, err := VerifyMetricoolDraftGrant(signed, trust, draftNow()); err != nil {
		t.Fatal(err)
	}
	signed.Grant.Action = "publish"
	if _, err := VerifyMetricoolDraftGrant(signed, trust, draftNow()); err == nil {
		t.Fatal("unapproved public permission accepted after tampering")
	}
	g.Action = "publish"
	if _, err := SignOwnerDraftGrant(g, priv, draftNow()); err == nil {
		t.Fatal("owner signing function granted publication action")
	}
	g.Action = "create-private-draft"
	g.ExpiresAt = draftNow().Add(-time.Minute).Format(time.RFC3339)
	if _, err := SignOwnerDraftGrant(g, priv, draftNow()); err == nil {
		t.Fatal("expired grant was signed")
	}
}
func TestOwnerRevokeIsIdempotentAndRejectsBadInputs(t *testing.T) {
	first, err := RevokeOwnerGrant([]string{}, "owner-approval-2026")
	if err != nil {
		t.Fatal(err)
	}
	second, err := RevokeOwnerGrant(first, "owner-approval-2026")
	if err != nil || len(second) != 1 {
		t.Fatal("revoke isn't idempotent")
	}
	if _, err := RevokeOwnerGrant(first, "evil/path"); err == nil {
		t.Fatal("path-like revocation accepted")
	}
	if _, err := RevokeOwnerGrant([]string{"owner-approval-2026", "owner-approval-2026"}, "other-valid-grant"); err == nil {
		t.Fatal("corrupt revocation list accepted")
	}
}
func TestOwnerCredentialPathRestrictions(t *testing.T) {
	root := t.TempDir()
	_, private := ownerTestKey()
	repo := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := WriteOwnerKeyExclusive(filepath.Join(repo, "owner.key"), private); err == nil {
		t.Fatal("stored signing key in Git repository")
	}
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "key-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadOwnerPrivateKey(link); err == nil {
		t.Fatal("read secret through symlink")
	}
}
