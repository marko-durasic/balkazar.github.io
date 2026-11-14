package gigops

// Owner-side approval helper. This MUST run in a human-controlled trust
// boundary whose signing key is inaccessible to the GigOps agent and provider
// runner. TTY prompts are UX only, not an authentication boundary.
import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func GenerateOwnerKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	return public, private, err
}
func SignOwnerDraftGrant(g MetricoolDraftGrant, private ed25519.PrivateKey, now time.Time) (SignedMetricoolDraftGrant, error) {
	if len(private) != ed25519.PrivateKeySize {
		return SignedMetricoolDraftGrant{}, errors.New("invalid owner signing key")
	}
	public := private.Public().(ed25519.PublicKey)
	payload, err := json.Marshal(g)
	if err != nil {
		return SignedMetricoolDraftGrant{}, err
	}
	signed := SignedMetricoolDraftGrant{
		Grant: g, Signature: base64.RawStdEncoding.EncodeToString(ed25519.Sign(private, payload)),
	}
	_, err = VerifyMetricoolDraftGrant(signed, MetricoolGrantTrust{
		OwnerPublicKey: public, RevokedGrantIDs: map[string]bool{},
	}, now)
	if err != nil {
		return SignedMetricoolDraftGrant{}, fmt.Errorf("owner grant cannot be issued: %w", err)
	}
	return signed, nil
}
func ValidateOwnerOnlyFile(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("owner key/policy file path must be absolute")
	}
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return errors.New("refusing owner credential/policy inside git checkout")
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	if st, err := os.Lstat(path); err == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			return errors.New("refusing owner key/policy symlink")
		}
		if st.Mode().Perm()&0077 != 0 {
			return errors.New("owner private file permissions must be 0600")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func WriteOwnerKeyExclusive(path string, private ed25519.PrivateKey) error {
	if err := ValidateOwnerOnlyFile(path); err != nil {
		return err
	}
	if len(private) != ed25519.PrivateKeySize {
		return errors.New("invalid signing key size")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, base64.RawStdEncoding.EncodeToString(private.Seed())); err != nil {
		return err
	}
	return f.Sync()
}
func ReadOwnerPrivateKey(path string) (ed25519.PrivateKey, error) {
	if err := ValidateOwnerOnlyFile(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 256 {
		return nil, errors.New("owner private key file is oversized")
	}
	seed, err := base64.RawStdEncoding.DecodeString(string(bytesTrimSpace(data)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("owner key seed invalid")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
func bytesTrimSpace(data []byte) []byte {
	for len(data) > 0 && (data[0] == ' ' || data[0] == '\n' || data[0] == '\r' || data[0] == '\t') {
		data = data[1:]
	}
	for len(data) > 0 && (data[len(data)-1] == ' ' || data[len(data)-1] == '\n' || data[len(data)-1] == '\r' || data[len(data)-1] == '\t') {
		data = data[:len(data)-1]
	}
	return data
}
func ValidateOwnerGrantRevocations(revocations []string) error {
	if len(revocations) > 1000 {
		return errors.New("revocation list too long")
	}
	seen := map[string]bool{}
	for _, id := range revocations {
		if !grantIDRE.MatchString(id) || seen[id] {
			return errors.New("invalid/duplicate owner revocation")
		}
		seen[id] = true
	}
	return nil
}
func RevokeOwnerGrant(existing []string, id string) ([]string, error) {
	if !grantIDRE.MatchString(id) {
		return nil, errors.New("invalid grant ID")
	}
	if err := ValidateOwnerGrantRevocations(existing); err != nil {
		return nil, err
	}
	out := append([]string(nil), existing...)
	for _, v := range out {
		if v == id {
			return out, nil
		}
	}
	out = append(out, id)
	return out, nil
}
