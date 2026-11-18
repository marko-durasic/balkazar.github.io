package main

// Owner-authorized PRIVATE DRAFT bridge runner.
//
// Deploy only as a dedicated OS service identity with an isolated OAuth token
// cache and root/operator-pinned config. Never expose this CLI as an arbitrary
// command-execution MCP to an LLM. No public publication is implemented.
import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	gigops "balkazar.org/musicops"
	"balkazar.org/musicops/mcp/clientprovider"
)

type operatorConfig struct {
	Version              int    `json:"version"`
	CampaignStore        string `json:"campaign_store"`
	OwnerSignedGrant     string `json:"owner_signed_grant"`
	OwnerPinnedPublicKey string `json:"owner_pinned_public_key"`
	OwnerRevocations     string `json:"owner_revocations"`
	ApprovedInbox        string `json:"approved_inbox"`
	OAuthProxyExecutable string `json:"oauth_proxy_executable"`
}

func loadFile(path string, max int) ([]byte, error) {
	stat, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if stat.Mode()&os.ModeSymlink != 0 || !stat.Mode().IsRegular() {
		return nil, errors.New("refusing non-regular file or symlink")
	}
	if stat.Size() > int64(max) {
		return nil, errors.New("input exceeds configured size limit")
	}
	return os.ReadFile(path)
}
func ownedProtectedFile(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("protected path must be absolute")
	}
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("protected file must be regular and not a symlink")
	}
	if st.Mode().Perm()&0077 != 0 {
		return errors.New("protected file must have 0600 permissions")
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || int(owner.Uid) != os.Geteuid() {
		return errors.New("protected file owner must be the isolated bridge service UID")
	}
	return nil
}
func parentIsProtected(path string) error {
	dir := filepath.Dir(path)
	st, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0022 != 0 {
		return errors.New("operator config directory must not be group/other writable or symlinked")
	}
	return nil
}
func decodeJSONStrict(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON or oversize input")
	}
	return nil
}
func readOperatorConfig(path string) (operatorConfig, error) {
	if err := parentIsProtected(path); err != nil {
		return operatorConfig{}, err
	}
	if err := ownedProtectedFile(path); err != nil {
		return operatorConfig{}, err
	}
	data, err := loadFile(path, 64<<10)
	if err != nil {
		return operatorConfig{}, err
	}
	var cfg operatorConfig
	if err := decodeJSONStrict(data, &cfg); err != nil {
		return operatorConfig{}, err
	}
	if cfg.Version != 1 || cfg.CampaignStore == "" || cfg.OwnerSignedGrant == "" ||
		cfg.OwnerPinnedPublicKey == "" || cfg.OwnerRevocations == "" ||
		cfg.ApprovedInbox == "" || cfg.OAuthProxyExecutable == "" {
		return operatorConfig{}, errors.New("missing required trusted bridge configuration")
	}
	for _, path := range []string{cfg.CampaignStore, cfg.OwnerSignedGrant, cfg.OwnerPinnedPublicKey, cfg.OwnerRevocations, cfg.ApprovedInbox, cfg.OAuthProxyExecutable} {
		if !filepath.IsAbs(path) {
			return operatorConfig{}, errors.New("all trusted paths must be absolute")
		}
	}
	dir, err := os.Lstat(cfg.ApprovedInbox)
	if err != nil {
		return operatorConfig{}, err
	}
	if !dir.IsDir() || dir.Mode()&os.ModeSymlink != 0 {
		return operatorConfig{}, errors.New("approved inbox is not a regular directory")
	}
	return cfg, nil
}
func readCandidate(cfg operatorConfig, path string) (gigops.MetricoolDraftCandidate, error) {
	full, err := filepath.Abs(path)
	if err != nil {
		return gigops.MetricoolDraftCandidate{}, err
	}
	root := filepath.Clean(cfg.ApprovedInbox)
	// Only a single file under an operator-approved inbox can be submitted.
	if filepath.Dir(full) != root {
		return gigops.MetricoolDraftCandidate{}, errors.New("candidate outside configured inbox")
	}
	data, err := loadFile(full, 64<<10)
	if err != nil {
		return gigops.MetricoolDraftCandidate{}, err
	}
	var c gigops.MetricoolDraftCandidate
	if err := decodeJSONStrict(data, &c); err != nil {
		return gigops.MetricoolDraftCandidate{}, err
	}
	return c, nil
}
func readTrust(cfg operatorConfig) (gigops.SignedMetricoolDraftGrant, gigops.MetricoolGrantTrust, error) {
	for _, path := range []string{cfg.OwnerSignedGrant, cfg.OwnerPinnedPublicKey, cfg.OwnerRevocations} {
		if err := ownedProtectedFile(path); err != nil {
			return gigops.SignedMetricoolDraftGrant{}, gigops.MetricoolGrantTrust{}, err
		}
	}
	grantBytes, err := loadFile(cfg.OwnerSignedGrant, 64<<10)
	if err != nil {
		return gigops.SignedMetricoolDraftGrant{}, gigops.MetricoolGrantTrust{}, err
	}
	var signed gigops.SignedMetricoolDraftGrant
	if err := decodeJSONStrict(grantBytes, &signed); err != nil {
		return signed, gigops.MetricoolGrantTrust{}, err
	}
	pubBytes, err := loadFile(cfg.OwnerPinnedPublicKey, 256)
	if err != nil {
		return signed, gigops.MetricoolGrantTrust{}, err
	}
	pub, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(pubBytes)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return signed, gigops.MetricoolGrantTrust{}, errors.New("owner public key invalid")
	}
	revBytes, err := loadFile(cfg.OwnerRevocations, 64<<10)
	if err != nil {
		return signed, gigops.MetricoolGrantTrust{}, err
	}
	var revokedIDs []string
	if err := decodeJSONStrict(revBytes, &revokedIDs); err != nil {
		return signed, gigops.MetricoolGrantTrust{}, err
	}
	trust := gigops.MetricoolGrantTrust{
		OwnerPublicKey:  ed25519.PublicKey(pub),
		RevokedGrantIDs: make(map[string]bool),
	}
	for _, id := range revokedIDs {
		trust.RevokedGrantIDs[id] = true
	}
	return signed, trust, nil
}
func run(args []string) error {
	flags := flag.NewFlagSet("musicops-metricool-bridge", flag.ContinueOnError)
	configPath := flags.String("config", "", "Protected service-owned operator config")
	candidatePath := flags.String("candidate", "", "One campaign draft candidate in the approved inbox")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *configPath == "" || *candidatePath == "" {
		return errors.New("--config and --candidate are required")
	}
	cfg, err := readOperatorConfig(*configPath)
	if err != nil {
		return err
	}
	candidate, err := readCandidate(cfg, *candidatePath)
	if err != nil {
		return err
	}
	signed, trust, err := readTrust(cfg)
	if err != nil {
		return err
	}
	// Fail before opening any OAuth network connection if approval is absent.
	if _, err := gigops.VerifyMetricoolDraftGrant(signed, trust, time.Now()); err != nil {
		return err
	}
	if err := ownedProtectedFile(cfg.CampaignStore); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// This invokes only the three pre-reviewed Metricool MCP tools.
	// OAuth login is one-time, in the owner's normal browser, from the service
	// identity that keeps all tokens OUT of the AI/agent's reach.
	provider, session, err := clientprovider.ConnectViaPinnedProxy(ctx, cfg.OAuthProxyExecutable)
	if err != nil {
		return fmt.Errorf("no authenticated/safe Metricool MCP provider: %w", err)
	}
	defer session.Close()
	result, err := gigops.RunTrustedMetricoolDraft(ctx, cfg.CampaignStore, candidate, signed, trust, provider, time.Now())
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err2 := encoder.Encode(result); err2 != nil && err == nil {
		err = err2
	}
	return err
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
