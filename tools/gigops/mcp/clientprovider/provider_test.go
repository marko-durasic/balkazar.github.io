package clientprovider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gigops "balkazar.org/musicops"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mockSession struct {
	tools       []*mcp.Tool
	calls       []string
	args        []any
	responses   map[string]any
	resultError bool
}

func (m *mockSession) ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{Tools: m.tools}, nil
}
func (m *mockSession) CallTool(_ context.Context, p *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	m.calls = append(m.calls, p.Name)
	m.args = append(m.args, p.Arguments)
	if _, ok := m.responses[p.Name]; !ok {
		return nil, errors.New("mock missing response")
	}
	return &mcp.CallToolResult{StructuredContent: m.responses[p.Name], IsError: m.resultError}, nil
}
func testSchema(keys ...string) any {
	props := map[string]any{}
	for _, key := range keys {
		props[key] = map[string]any{"type": "string"}
	}
	return map[string]any{"type": "object", "properties": props}
}
func legacySession() *mockSession {
	return &mockSession{
		tools: []*mcp.Tool{
			{Name: "getBrandSettings", InputSchema: testSchema()},
			{Name: "getScheduledPosts", InputSchema: testSchema("brandId", "fromDate", "toDate", "timezone")},
			{Name: "createScheduledPost", InputSchema: testSchema("date", "blogId", "info")},
		},
		responses: map[string]any{
			"getBrandSettings": map[string]any{"data": []any{map[string]any{
				"id": 7273414, "label": "balkazarband", "timezone": "Asia/Taipei",
				"networksData": map[string]any{
					"facebookData": "1790690501208601", "instagramData": "balkazarband", "threadsData": "balkazarband",
				},
			}}},
			"getScheduledPosts": map[string]any{"data": []any{}},
			"createScheduledPost": map[string]any{"data": map[string]any{
				"id": 421790030, "uuid": "bridge-private-draft-12345",
			}},
		},
	}
}
func sampleArgs() gigops.MetricoolDraftArguments {
	info := map[string]any{
		"draft": true, "autoPublish": false,
		"text":  "Balkazar live at European Christmas Market, December 3! https://balkazar.org/events/2026-12-03-european-christmas-market/",
		"media": []string{"https://balkazar.org/assets/images/gallery/g-christmas-market-horns.jpg"},
		"providers": []map[string]string{
			{"network": "facebook"}, {"network": "instagram"}, {"network": "threads"},
		},
	}
	b, _ := json.Marshal(info)
	return gigops.MetricoolDraftArguments{BlogID: "7273414", Date: "2026-11-24T19:00:00+08:00", Info: string(b)}
}
func TestLegacyAuthenticatedMCPProviderPreservesPrivateDraftFlags(t *testing.T) {
	s := legacySession()
	p, err := NewProvider(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	brand, err := p.Brand(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if brand.ID != "7273414" || brand.Name != "balkazarband" || len(brand.Networks) != 3 {
		t.Fatalf("wrong authenticated brand %+v", brand)
	}
	posts, err := p.ScheduledPosts(context.Background(), brand.ID, time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if string(posts) == "" {
		t.Fatal("no planner JSON")
	}
	receipt, err := p.CreatePrivateDraft(context.Background(), sampleArgs())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.UUID != "bridge-private-draft-12345" || receipt.PostID != "421790030" {
		t.Fatalf("wrong stable receipt %+v", receipt)
	}
	if len(s.calls) != 3 || s.calls[0] != "getBrandSettings" || s.calls[1] != "getScheduledPosts" || s.calls[2] != "createScheduledPost" {
		t.Fatalf("unexpected MCP methods called: %v", s.calls)
	}
	last := s.args[2].(map[string]any)
	if last["blogId"] != "7273414" {
		t.Fatal("wrong brand in create")
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(last["info"].(string)), &info); err != nil {
		t.Fatal(err)
	}
	if info["autoPublish"] != false || info["draft"] != true {
		t.Fatal("allowed unsafe public create")
	}
}
func TestMCPProviderRejectsHostedWithoutExplicitDraftSwitches(t *testing.T) {
	s := legacySession()
	s.tools = []*mcp.Tool{
		{Name: "get_brand_settings", InputSchema: testSchema()},
		{Name: "get_scheduled_posts", InputSchema: testSchema("brand_id", "from_date", "to_date", "timezone")},
		{Name: "create_scheduled_post", InputSchema: testSchema("blog_id", "date", "timezone", "networks", "text", "media")},
	}
	if _, err := NewProvider(context.Background(), s); err == nil {
		t.Fatal("hosted scheduling-only tool treated as a safe draft publisher")
	}
	if len(s.calls) != 0 {
		t.Fatal("called tool before validating schema")
	}
}
func TestMCPProviderSupportsHostedOnlyWhenBothSafetyFlagsExist(t *testing.T) {
	s := legacySession()
	s.tools = []*mcp.Tool{
		{Name: "get_brand_settings", InputSchema: testSchema()},
		{Name: "get_scheduled_posts", InputSchema: testSchema("brand_id", "from_date", "to_date", "timezone")},
		{Name: "create_scheduled_post", InputSchema: testSchema("blog_id", "date", "timezone", "networks", "text", "media", "draft", "auto_publish")},
	}
	s.responses["create_scheduled_post"] = s.responses["createScheduledPost"]
	p, err := NewProvider(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := p.CreatePrivateDraft(context.Background(), sampleArgs())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.UUID == "" {
		t.Fatal("missing provider receipt")
	}
	last := s.args[len(s.args)-1].(map[string]any)
	if last["draft"] != true || last["auto_publish"] != false {
		t.Fatal("hosted adapter removed safety flags")
	}
}
func TestMCPProviderRejectsUnsafeBodyAndProviderError(t *testing.T) {
	s := legacySession()
	p, err := NewProvider(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	args := sampleArgs()
	args.Info = `{"draft":false,"autoPublish":true}`
	if _, err := p.CreatePrivateDraft(context.Background(), args); err == nil {
		t.Fatal("provider attempted to publish without private-draft flags")
	}
	if len(s.calls) != 0 {
		t.Fatal("unsafe payload reached remote tool")
	}
	s.resultError = true
	if _, err := p.Brand(context.Background()); err == nil {
		t.Fatal("MCP isError ignored")
	}
}
func TestMCPProviderRejectsUnrecognizedResponses(t *testing.T) {
	s := legacySession()
	s.responses["createScheduledPost"] = map[string]any{"data": map[string]any{"plannerUrl": "https://app.metricool.com/foo"}}
	p, err := NewProvider(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CreatePrivateDraft(context.Background(), sampleArgs()); err == nil {
		t.Fatal("non-UUID response treated as verified")
	}
}
