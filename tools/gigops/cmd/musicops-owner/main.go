package main

// Human OWNER-ONLY signing tool. It must run under an isolated OS identity,
// outside any agent's remote-shell/MCP access. An interactive prompt alone
// does NOT establish human identity; the OS key boundary is mandatory.
import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gigops "balkazar.org/musicops"
)

func ownerConfirm(expected, description string) error {
	st, err := os.Stdin.Stat()
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeCharDevice == 0 {
		return errors.New("owner approval requires an interactive terminal (not a pipe or agent job)")
	}
	fmt.Fprintln(os.Stderr, description)
	fmt.Fprintf(os.Stderr, "Type %q to confirm: ", expected)
	reply, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply) != expected {
		return errors.New("owner confirmation not received")
	}
	return nil
}
func readBoundedJSON(path string, v any) error {
	if err := gigops.ValidateOwnerOnlyFile(path); err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(contents) > 1<<20 {
		return errors.New("owner policy file exceeds one MiB")
	}
	return json.Unmarshal(contents, v)
}
func writeOwnerFileExclusive(path string, contents []byte) error {
	if err := gigops.ValidateOwnerOnlyFile(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(contents); err != nil {
		return err
	}
	return f.Sync()
}
func writeOwnerFileAtomic(path string, contents []byte) error {
	if err := gigops.ValidateOwnerOnlyFile(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".owner-revocations-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(contents); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func initialize(args []string) error {
	f := flag.NewFlagSet("init-key", flag.ContinueOnError)
	privatePath := f.String("private-key", "", "Owner-only absolute private seed path outside agent access")
	publicPath := f.String("public-key", "", "Pinned public key file destination")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *privatePath == "" || *publicPath == "" || *privatePath == *publicPath {
		return errors.New("distinct absolute private-key and public-key destinations required")
	}
	if _, err := os.Lstat(*privatePath); !errors.Is(err, os.ErrNotExist) {
		return errors.New("private key already exists or path inaccessible; refusing overwrite")
	}
	if _, err := os.Lstat(*publicPath); !errors.Is(err, os.ErrNotExist) {
		return errors.New("public key already exists or path inaccessible; refusing overwrite")
	}
	if err := ownerConfirm("GENERATE MUSICOPS OWNER KEY",
		"Generate a NEW signing key ONLY in a separate human-controlled account, not accessible by Desktop Commander, agents or CI."); err != nil {
		return err
	}
	public, private, err := gigops.GenerateOwnerKeypair()
	if err != nil {
		return err
	}
	if err := gigops.WriteOwnerKeyExclusive(*privatePath, private); err != nil {
		return err
	}
	if err := writeOwnerFileExclusive(*publicPath, []byte(base64.RawStdEncoding.EncodeToString(public)+"\n")); err != nil {
		return fmt.Errorf("private key was created but public key write failed; securely recover or rotate: %w", err)
	}
	fmt.Fprintln(os.Stdout, "Owner signing key created privately. Public key written. Never give the private key or seed to an agent.")
	return nil
}
func issue(args []string) error {
	f := flag.NewFlagSet("issue", flag.ContinueOnError)
	keyPath := f.String("private-key", "", "Human-owned seed path")
	grantPath := f.String("grant", "", "Reviewed unsigned draft-only policy JSON")
	outPath := f.String("out", "", "New signed grant file (exclusive create)")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *keyPath == "" || *grantPath == "" || *outPath == "" {
		return errors.New("--private-key, --grant and --out required")
	}
	var grant gigops.MetricoolDraftGrant
	if err := readBoundedJSON(*grantPath, &grant); err != nil {
		return err
	}
	description := fmt.Sprintf(
		"\nMUSICOPS OWNER REVIEW — PRIVATE DRAFTS ONLY\nGrant: %s\nArtist: %s / Brand: %s\nGig: %s\nAction: %s\nNetworks: %s\nAllowed media: %v\nFrom: %s\nUntil: %s\nMax drafts: %d\nSpend: NT$%d\nNo scheduling/public publishing permission is granted.\n",
		grant.GrantID, grant.Artist, grant.BrandID, grant.GigID, grant.Action,
		strings.Join(grant.Networks, ", "), grant.MediaURLAllowlist, grant.ValidFrom,
		grant.ExpiresAt, grant.MaxDrafts, grant.SpendLimitNTD)
	if err := ownerConfirm("APPROVE "+grant.GrantID, description); err != nil {
		return err
	}
	private, err := gigops.ReadOwnerPrivateKey(*keyPath)
	if err != nil {
		return err
	}
	signed, err := gigops.SignOwnerDraftGrant(grant, private, time.Now())
	if err != nil {
		return err
	}
	contents, err := json.MarshalIndent(signed, "", "  ")
	if err != nil {
		return err
	}
	if err := writeOwnerFileExclusive(*outPath, append(contents, '\n')); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Signed draft-only grant saved. Keep revoke controls outside agent access.")
	return nil
}
func revoke(args []string) error {
	f := flag.NewFlagSet("revoke", flag.ContinueOnError)
	listPath := f.String("revocations", "", "Owner-only revocation list")
	grantID := f.String("grant-id", "", "Grant to revoke")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *listPath == "" || *grantID == "" {
		return errors.New("--revocations and --grant-id required")
	}
	ids := []string{}
	if err := readBoundedJSON(*listPath, &ids); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	next, err := gigops.RevokeOwnerGrant(ids, *grantID)
	if err != nil {
		return err
	}
	if err := ownerConfirm("REVOKE "+*grantID, "Revoke this grant so all future draft creation attempts are denied. Existing private drafts are not deleted."); err != nil {
		return err
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := writeOwnerFileAtomic(*listPath, append(b, '\n')); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Grant ID added to owner revocation list.")
	return nil
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: musicops-owner init-key|issue|revoke [flags]")
	}
	switch args[0] {
	case "init-key":
		return initialize(args[1:])
	case "issue":
		return issue(args[1:])
	case "revoke":
		return revoke(args[1:])
	default:
		return errors.New("unknown owner-only operation")
	}
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
