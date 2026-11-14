package gigops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockMetricool struct {
	mu               sync.Mutex
	brand            ConnectedMetricoolBrand
	posts            []map[string]any
	creates          int
	failCreate       bool
	createdButFailed bool
	hideNewPost      bool
	blockCreate      chan struct{}
}

func newMockMetricool() *mockMetricool {
	return &mockMetricool{posts: []map[string]any{}, brand: ConnectedMetricoolBrand{
		ID: balkazarMetricoolBrandID, Name: "balkazarband", Timezone: "Asia/Taipei",
		Networks: []string{"facebook", "instagram", "threads"},
	}}
}
func (p *mockMetricool) Brand(context.Context) (ConnectedMetricoolBrand, error) {
	return p.brand, nil
}
func (p *mockMetricool) ScheduledPosts(_ context.Context, _ string, _, _ time.Time) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	payload, _ := json.Marshal(map[string]any{"data": p.posts})
	return payload, nil
}
func (p *mockMetricool) CreatePrivateDraft(_ context.Context, args MetricoolDraftArguments) (MetricoolDraftReceipt, error) {
	if err := validateDraftOnlyArgs(args); err != nil {
		return MetricoolDraftReceipt{}, err
	}
	if p.blockCreate != nil {
		<-p.blockCreate
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.creates++
	if p.failCreate && !p.createdButFailed {
		return MetricoolDraftReceipt{}, errors.New("provider temporarily unavailable")
	}
	var info struct {
		Text            string   `json:"text"`
		Media           []string `json:"media"`
		Draft           bool     `json:"draft"`
		AutoPublish     bool     `json:"autoPublish"`
		PublicationDate struct {
			DateTime string `json:"dateTime"`
			Timezone string `json:"timezone"`
		} `json:"publicationDate"`
		Providers []struct {
			Network string `json:"network"`
		} `json:"providers"`
	}
	if err := json.Unmarshal([]byte(args.Info), &info); err != nil {
		return MetricoolDraftReceipt{}, err
	}
	if !info.Draft || info.AutoPublish {
		return MetricoolDraftReceipt{}, errors.New("test detected external publication attempt")
	}
	receipt := MetricoolDraftReceipt{UUID: fmt.Sprintf("bridge-test-uuid-%d", p.creates), PostID: fmt.Sprintf("%d", 421700000+p.creates)}
	if !p.hideNewPost {
		providers := []map[string]string{}
		for _, n := range info.Providers {
			providers = append(providers, map[string]string{"network": n.Network, "status": "PENDING"})
		}
		p.posts = append(p.posts, map[string]any{
			"id": 421700000 + p.creates, "uuid": receipt.UUID, "draft": true, "autoPublish": false,
			"text": info.Text, "media": info.Media, "providers": providers,
			"publicationDate": map[string]string{"dateTime": info.PublicationDate.DateTime, "timezone": info.PublicationDate.Timezone},
		})
	}
	if p.failCreate {
		return MetricoolDraftReceipt{}, errors.New("provider returned timeout after possible creation")
	}
	return receipt, nil
}
func testBridgeSetup(t *testing.T) (string, MetricoolDraftCandidate, SignedMetricoolDraftGrant, MetricoolGrantTrust) {
	t.Helper()
	s, c, g, trust, _ := draftTestSetup(t)
	path := privateStorePath(t)
	if err := SaveCampaignStore(path, s); err != nil {
		t.Fatal(err)
	}
	return path, c, g, trust
}

func TestTrustedDraftBridgeCreatesExactlyOneVerifiedPrivateDraft(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	provider := newMockMetricool()
	result, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, provider, draftNow())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "draft-confirmed" || result.PublicWrites != 0 || result.DraftWrites != 1 || result.ProviderUUID == "" {
		t.Fatalf("unexpected bridge result %+v", result)
	}
	if provider.creates != 1 {
		t.Fatal("wrong number of provider draft calls")
	}
	s, err := LoadCampaignStore(path)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := s.DraftAttempts[result.IntentKey]
	if !ok || a.State != "draft-confirmed" || a.UUID != result.ProviderUUID {
		t.Fatal("verified provider receipt not durable")
	}
	again, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, provider, draftNow())
	if err != nil || again.Status != "blocked-existing" || again.DraftWrites != 0 {
		t.Fatalf("existing draft not safely recognized: %+v error %v", again, err)
	}
	if provider.creates != 1 {
		t.Fatal("replayed write after confirmed receipt")
	}
}
func TestTrustedDraftBridgeAmbiguousProviderFailureNeverRetries(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	provider := newMockMetricool()
	provider.failCreate = true
	provider.createdButFailed = true
	result, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, provider, draftNow())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "uncertain" || result.DraftWrites != 1 {
		t.Fatalf("uncertain call mishandled %+v", result)
	}
	if provider.creates != 1 {
		t.Fatal("unexpected create call count")
	}
	_, err = RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, provider, draftNow())
	if err == nil {
		t.Fatal("ambiguous create was automatically retried")
	}
	if provider.creates != 1 {
		t.Fatal("duplicate write after uncertain result")
	}
	reconciled, err := ReconcileDraftAttempt(context.Background(), path, result.IntentKey, provider, draftNow().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Status != "draft-confirmed" {
		t.Fatalf("failed to reconcile existing provider post %+v", reconciled)
	}
	if provider.creates != 1 {
		t.Fatal("reconciliation created another draft")
	}
}
func TestBridgeWithoutVerifiedReadbackStaysUncertain(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	p := newMockMetricool()
	p.hideNewPost = true
	result, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "uncertain" || p.creates != 1 {
		t.Fatal("unverified result marked confirmed")
	}
	reconciled, err := ReconcileDraftAttempt(context.Background(), path, result.IntentKey, p, draftNow().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Status != "uncertain" {
		t.Fatalf("did not preserve uncertain state %+v", reconciled)
	}
	_, err = RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow())
	if err == nil || p.creates != 1 {
		t.Fatal("missing readback caused duplicate create")
	}
}
func TestBridgeBlocksWrongBrandAndRevokedGrantBeforeProviderWrite(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	p := newMockMetricool()
	p.brand.ID = "wrong-brand"
	if _, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow()); err == nil {
		t.Fatal("wrong connected brand authorized")
	}
	if p.creates != 0 {
		t.Fatal("called unauthorized provider")
	}
	p.brand.ID = balkazarMetricoolBrandID
	trust.RevokedGrantIDs[grant.Grant.GrantID] = true
	if _, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow()); err == nil {
		t.Fatal("revoked owner grant authorized")
	}
	if p.creates != 0 {
		t.Fatal("called provider despite revocation")
	}
	s, err := LoadCampaignStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DraftAttempts) != 0 {
		t.Fatal("invalid attempt recorded")
	}
}
func TestBridgeExistingPlannerDraftBlocksDuplicateWithoutCreate(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	p := newMockMetricool()
	providers := []map[string]string{}
	for _, n := range c.Networks {
		providers = append(providers, map[string]string{"network": n, "status": "PENDING"})
	}
	p.posts = []map[string]any{{
		"id": 421679435, "uuid": "6368837845729689867", "draft": true, "autoPublish": false,
		"text": c.Text, "media": c.Media, "providers": providers,
		"publicationDate": map[string]string{"dateTime": "2026-11-24T19:00:00", "timezone": "Asia/Taipei"},
	}}
	r, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "blocked-existing" || r.DraftWrites != 0 || p.creates != 0 {
		t.Fatalf("duplicate was created or not recognized: %+v", r)
	}
}
func TestBridgeRescheduleRequiresFreshGrantAndRevision(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	s, err := LoadCampaignStore(path)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Campaigns[c.GigID].Gig
	g.Start = "2026-12-04T20:00:00+08:00"
	g.End = "2026-12-04T21:30:00+08:00"
	if _, err := s.ReconcileGig(g, draftNow().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := SaveCampaignStore(path, s); err != nil {
		t.Fatal(err)
	}
	p := newMockMetricool()
	if _, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow()); err == nil {
		t.Fatal("stale source revision authorized draft")
	}
	if p.creates != 0 {
		t.Fatal("write on stale gig")
	}
}
func TestBridgeRespectsLockAndNeverDispatchesConcurrentCopy(t *testing.T) {
	path, c, grant, trust := testBridgeSetup(t)
	p := newMockMetricool()
	p.blockCreate = make(chan struct{})
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(entered)
		_, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow())
		done <- err
	}()
	<-entered
	// Wait until reservation is persisted (deterministically, not a sleep
	// controlling provider behavior); cap the wait to protect test runners.
	limit := time.Now().Add(2 * time.Second)
	for {
		s, err := LoadCampaignStore(path)
		if err == nil && len(s.DraftAttempts) > 0 {
			break
		}
		if time.Now().After(limit) {
			close(p.blockCreate)
			t.Fatal("bridge never dispatched")
		}
		time.Sleep(time.Millisecond)
	}
	_, err := RunTrustedMetricoolDraft(context.Background(), path, c, grant, trust, p, draftNow())
	if err == nil || !strings.Contains(err.Error(), "already locked") {
		close(p.blockCreate)
		t.Fatalf("concurrent operation not denied: %v", err)
	}
	close(p.blockCreate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if p.creates != 1 {
		t.Fatalf("duplicate concurrent create count %d", p.creates)
	}
}
