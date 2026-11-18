package gigops

// Trusted draft bridge: a provider interface owned by an authenticated runtime,
// not caller-supplied JSON or a model-generated authentication token. All
// external writes are restricted to Metricool PRIVATE drafts. A reservation is
// persisted before the single create attempt; ambiguous outcomes are NEVER
// automatically retried.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type ConnectedMetricoolBrand struct {
	ID       string
	Name     string
	Timezone string
	Networks []string
}

// MetricoolDraftProvider must be implemented by a trusted authenticated
// connector-side adapter; a fake is used in tests. A raw caller-supplied JSON
// snapshot or command-line flag is not an implementation of this contract.
type MetricoolDraftProvider interface {
	Brand(context.Context) (ConnectedMetricoolBrand, error)
	ScheduledPosts(context.Context, string, time.Time, time.Time) ([]byte, error)
	CreatePrivateDraft(context.Context, MetricoolDraftArguments) (MetricoolDraftReceipt, error)
}

type MetricoolDraftReceipt struct {
	UUID   string
	PostID string
}

type BridgeDraftAttempt struct {
	IntentKey      string   `json:"intent_key"`
	GrantID        string   `json:"grant_id"`
	GigID          string   `json:"gig_id"`
	SourceRevision uint64   `json:"source_revision"`
	ContentSHA256  string   `json:"content_sha256"`
	PublicationAt  string   `json:"publication_at"`
	Networks       []string `json:"networks"`
	State          string   `json:"state"` // dispatching, uncertain, draft-confirmed
	UUID           string   `json:"uuid,omitempty"`
	PostID         string   `json:"post_id,omitempty"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type BridgeDraftResult struct {
	Status       string `json:"status"`
	IntentKey    string `json:"intent_key,omitempty"`
	ProviderUUID string `json:"provider_uuid,omitempty"`
	PublicWrites int    `json:"public_writes"` // Always 0. A private draft is not a public post.
	DraftWrites  int    `json:"draft_writes"`  // 0 or 1 attempted
	Reason       string `json:"reason,omitempty"`
}

func validateBridgeAttempts(s CampaignStore) error {
	if len(s.DraftAttempts) > 300 {
		return errors.New("too many draft attempts")
	}
	for key, a := range s.DraftAttempts {
		if !storedFingerprintRE.MatchString(key) || a.IntentKey != key ||
			!grantIDRE.MatchString(a.GrantID) ||
			!durableGigID.MatchString(a.GigID) ||
			a.SourceRevision == 0 ||
			!storedFingerprintRE.MatchString(a.ContentSHA256) ||
			!allowed(a.State, "dispatching", "uncertain", "draft-confirmed") {
			return errors.New("invalid draft execution state")
		}
		if _, ok := s.Campaigns[a.GigID]; !ok {
			return errors.New("draft attempt refers to missing gig")
		}
		if _, err := time.Parse(time.RFC3339, a.CreatedAt); err != nil {
			return errors.New("invalid draft attempt time")
		}
		if _, err := time.Parse(time.RFC3339, a.UpdatedAt); err != nil {
			return errors.New("invalid draft update time")
		}
		if _, err := time.Parse(time.RFC3339, a.PublicationAt); err != nil {
			return errors.New("invalid draft scheduled time")
		}
		if a.State == "draft-confirmed" && (!metricoolUUIDRE.MatchString(a.UUID) || !metricoolPostIDRE.MatchString(a.PostID)) {
			return errors.New("confirmed draft lacks provider receipt")
		}
	}
	return nil
}

func bridgeGrantUsage(s CampaignStore, grantID string) GrantUsage {
	u := GrantUsage{GrantID: grantID, DraftUUIDs: []string{}, InFlightKeys: []string{}}
	for key, a := range s.DraftAttempts {
		if a.GrantID != grantID {
			continue
		}
		if a.State == "draft-confirmed" {
			u.DraftUUIDs = append(u.DraftUUIDs, a.UUID)
		} else {
			u.InFlightKeys = append(u.InFlightKeys, key)
		}
	}
	sort.Strings(u.DraftUUIDs)
	sort.Strings(u.InFlightKeys)
	return u
}

func validateConnectedBrand(b ConnectedMetricoolBrand, networks []string) error {
	if b.ID != balkazarMetricoolBrandID || b.Timezone != "Asia/Taipei" ||
		!strings.EqualFold(b.Name, "balkazarband") {
		return errors.New("authenticated Metricool brand is not the authorized Balkazar account")
	}
	for _, network := range networks {
		if !allowed(network, b.Networks...) {
			return fmt.Errorf("authenticated Metricool brand lacks connected network %s", network)
		}
	}
	return nil
}

func readActualPlanner(ctx context.Context, provider MetricoolDraftProvider, c StoredCampaign, from, to time.Time) (MetricoolReadbackPreview, error) {
	raw, err := provider.ScheduledPosts(ctx, balkazarMetricoolBrandID, from, to)
	if err != nil {
		return MetricoolReadbackPreview{}, err
	}
	return PreviewMetricoolSnapshot(bytes.NewReader(raw), c.Gig, balkazarMetricoolBrandID)
}

func withCampaignLock(path string, fn func(*CampaignStore) (BridgeDraftResult, error)) (BridgeDraftResult, error) {
	if err := checkCampaignFilePath(path); err != nil {
		return BridgeDraftResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return BridgeDraftResult{}, err
	}
	lockPath := path + ".lock"
	if st, err := os.Lstat(lockPath); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return BridgeDraftResult{}, errors.New("refusing symlink lock path")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return BridgeDraftResult{}, err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return BridgeDraftResult{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return BridgeDraftResult{}, fmt.Errorf("campaign store already locked: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	s, err := LoadCampaignStore(path)
	if err != nil {
		return BridgeDraftResult{}, err
	}
	return fn(&s)
}

// RunTrustedMetricoolDraft performs the full "preflight -> reserve -> exactly
// one create -> readback" loop. The owner grant, trusted key and provider must
// come from independently configured operator-controlled surfaces. This call
// MUST NOT be exposed as a generic agent-writable MCP method.
func RunTrustedMetricoolDraft(ctx context.Context, path string, candidate MetricoolDraftCandidate,
	signed SignedMetricoolDraftGrant, trust MetricoolGrantTrust,
	provider MetricoolDraftProvider, now time.Time) (BridgeDraftResult, error) {
	if provider == nil {
		return BridgeDraftResult{}, errors.New("no trusted provider adapter")
	}
	return withCampaignLock(path, func(s *CampaignStore) (BridgeDraftResult, error) {
		c, found := s.Campaigns[candidate.GigID]
		if !found {
			return BridgeDraftResult{}, errors.New("campaign not found")
		}
		if err := validateBridgeAttempts(*s); err != nil {
			return BridgeDraftResult{}, err
		}
		g, err := VerifyMetricoolDraftGrant(signed, trust, now)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		usage := bridgeGrantUsage(*s, g.GrantID)
		// Validate proposed content, source and grant before asking the
		// connected provider for any account data.
		previewIntent, err := BuildMetricoolDraftIntent(*s, candidate, signed, trust, usage, nil, now)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		if previewIntent.Status != "needs-live-readback" || previewIntent.CreatePostArguments == nil {
			return BridgeDraftResult{Status: "blocked-existing", IntentKey: previewIntent.IntentKey, Reason: previewIntent.Blocker}, nil
		}
		b, err := provider.Brand(ctx)
		if err != nil {
			return BridgeDraftResult{}, fmt.Errorf("brand check failed: %w", err)
		}
		if err := validateConnectedBrand(b, candidate.Networks); err != nil {
			return BridgeDraftResult{}, err
		}
		scheduledAt, err := time.Parse(time.RFC3339, candidate.PublicationAt)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		from := now.Add(-24 * time.Hour)
		to := scheduledAt.Add(24 * time.Hour)
		if to.Before(from) {
			to = from.Add(24 * time.Hour)
		}
		livePreview, err := readActualPlanner(ctx, provider, c, from, to)
		if err != nil {
			return BridgeDraftResult{}, fmt.Errorf("live planner check failed: %w", err)
		}
		intent, err := BuildMetricoolDraftIntent(*s, candidate, signed, trust, usage, &livePreview, now)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		if intent.Status != "needs-live-readback" || intent.CreatePostArguments == nil {
			return BridgeDraftResult{Status: "blocked-existing", IntentKey: intent.IntentKey,
				ProviderUUID: intent.ExistingUUID, Reason: intent.Blocker}, nil
		}
		if _, exists := s.DraftAttempts[intent.IntentKey]; exists {
			return BridgeDraftResult{Status: "blocked-previous-attempt", IntentKey: intent.IntentKey,
				Reason: "prior attempt exists; reconcile from provider before considering any new create"}, nil
		}
		// The only allowed outbound write has both draft and autoPublish
		// flags pinned. A provider adapter must validate these again.
		if err := validateDraftOnlyArgs(*intent.CreatePostArguments); err != nil {
			return BridgeDraftResult{}, err
		}
		if s.DraftAttempts == nil {
			s.DraftAttempts = map[string]BridgeDraftAttempt{}
		}
		networks, _ := canonicalNetworks(candidate.Networks)
		_, _, contentHash, err := preflightDraftData(c, candidate, now)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		record := BridgeDraftAttempt{IntentKey: intent.IntentKey, GrantID: g.GrantID,
			GigID: candidate.GigID, SourceRevision: c.Revision, ContentSHA256: contentHash,
			PublicationAt: scheduledAt.Format(time.RFC3339), Networks: networks,
			State: "dispatching", CreatedAt: now.UTC().Format(time.RFC3339),
			UpdatedAt: now.UTC().Format(time.RFC3339)}
		s.DraftAttempts[intent.IntentKey] = record
		if err := SaveCampaignStore(path, *s); err != nil {
			return BridgeDraftResult{}, err
		}
		// From this point, a crash or ambiguous provider error blocks all
		// automatic retries: the durable record was persisted first.
		result := BridgeDraftResult{Status: "uncertain", IntentKey: intent.IntentKey,
			DraftWrites: 1, Reason: "Provider call may have succeeded; no blind retry"}
		receipt, createErr := provider.CreatePrivateDraft(ctx, *intent.CreatePostArguments)
		if createErr == nil {
			// Read back directly from trusted provider, not a model-supplied
			// UUID or an external saved JSON snapshot.
			confirm, readErr := readActualPlanner(ctx, provider, c, from, to)
			if readErr == nil && metricoolDraftReceiptMatches(confirm, receipt, contentHash, scheduledAt, networks) {
				record.State = "draft-confirmed"
				record.UUID = receipt.UUID
				record.PostID = receipt.PostID
				result.Status = "draft-confirmed"
				result.ProviderUUID = receipt.UUID
				result.Reason = "Confirmed private draft in fresh authenticated Metricool planner readback"
			}
		}
		record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		s.DraftAttempts[intent.IntentKey] = record
		if err := SaveCampaignStore(path, *s); err != nil {
			return BridgeDraftResult{Status: "uncertain", IntentKey: intent.IntentKey, DraftWrites: 1,
				Reason: "Draft call attempted; local final receipt write failed, do not retry"}, err
		}
		return result, nil
	})
}

func validateDraftOnlyArgs(args MetricoolDraftArguments) error {
	if args.BlogID != balkazarMetricoolBrandID || args.Date == "" || args.Info == "" {
		return errors.New("invalid Metricool draft connector request identity")
	}
	var payload struct {
		Draft       bool `json:"draft"`
		AutoPublish bool `json:"autoPublish"`
	}
	if err := json.Unmarshal([]byte(args.Info), &payload); err != nil {
		return err
	}
	if !payload.Draft || payload.AutoPublish {
		return errors.New("bridge refuses any non-draft or auto-publish request")
	}
	return nil
}

func metricoolDraftReceiptMatches(p MetricoolReadbackPreview, receipt MetricoolDraftReceipt,
	hash string, when time.Time, networks []string) bool {
	if !metricoolUUIDRE.MatchString(receipt.UUID) || !metricoolPostIDRE.MatchString(receipt.PostID) {
		return false
	}
	for _, item := range p.MatchedPosts {
		if item.PostUUID == receipt.UUID && item.PlannerPostID == receipt.PostID &&
			item.State == "observed-draft" && !item.AutoPublish &&
			item.ContentSHA256 == hash && item.PublicationAt == when.Format(time.RFC3339) &&
			sameStringList(item.Networks, networks) {
			return true
		}
	}
	return false
}

// ReconcileDraftAttempt NEVER invokes CreatePrivateDraft. It can confirm a
// previously ambiguous attempt from fresh provider data, or leave it blocked.
func ReconcileDraftAttempt(ctx context.Context, path, intentKey string,
	provider MetricoolDraftProvider, now time.Time) (BridgeDraftResult, error) {
	if provider == nil {
		return BridgeDraftResult{}, errors.New("provider required")
	}
	return withCampaignLock(path, func(s *CampaignStore) (BridgeDraftResult, error) {
		record, ok := s.DraftAttempts[intentKey]
		if !ok {
			return BridgeDraftResult{}, errors.New("unknown intent")
		}
		if record.State == "draft-confirmed" {
			return BridgeDraftResult{Status: "draft-confirmed", IntentKey: intentKey, ProviderUUID: record.UUID}, nil
		}
		c, ok := s.Campaigns[record.GigID]
		if !ok {
			return BridgeDraftResult{}, errors.New("source gig missing")
		}
		brand, err := provider.Brand(ctx)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		if err := validateConnectedBrand(brand, record.Networks); err != nil {
			return BridgeDraftResult{}, err
		}
		if record.SourceRevision != c.Revision {
			return BridgeDraftResult{Status: "stale-review-required", IntentKey: intentKey, Reason: "gig changed; review against former event before further actions"}, nil
		}
		from := now.Add(-24 * time.Hour)
		when, err := time.Parse(time.RFC3339, record.PublicationAt)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		to := when.Add(24 * time.Hour)
		if to.Before(from) {
			to = from.Add(24 * time.Hour)
		}
		snapshot, err := readActualPlanner(ctx, provider, c, from, to)
		if err != nil {
			return BridgeDraftResult{}, err
		}
		var candidates []MetricoolPostObservation
		for _, p := range snapshot.MatchedPosts {
			if p.State == "observed-draft" && !p.AutoPublish && p.ContentSHA256 == record.ContentSHA256 &&
				p.PublicationAt == record.PublicationAt && sameStringList(p.Networks, record.Networks) {
				candidates = append(candidates, p)
			}
		}
		if len(candidates) != 1 {
			return BridgeDraftResult{Status: "uncertain", IntentKey: intentKey,
				Reason: "No unique private draft matching the uncertain intent; do not automatically retry"}, nil
		}
		record.UUID = candidates[0].PostUUID
		record.PostID = candidates[0].PlannerPostID
		record.State = "draft-confirmed"
		record.UpdatedAt = now.UTC().Format(time.RFC3339)
		s.DraftAttempts[intentKey] = record
		if err := SaveCampaignStore(path, *s); err != nil {
			return BridgeDraftResult{}, err
		}
		return BridgeDraftResult{Status: "draft-confirmed", IntentKey: intentKey, ProviderUUID: record.UUID}, nil
	})
}
