package clientprovider

// Restricted real Metricool MCP adapter. Authentication belongs to an
// independent, owner-approved OAuth client. No tool name or endpoint may be
// selected by an agent. Unsupported schemas fail closed rather than risking a
// public post. This package never requests OAuth credentials from the model.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gigops "balkazar.org/musicops"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const hostedMetricoolMCP = "https://ai.metricool.com/mcp"
const maxToolResultBytes = 4 << 20

// Session is purposely tiny so the restricted provider can be tested without
// an authenticated connection or any live social-network side effects.
type Session interface {
	ListTools(context.Context, *mcp.ListToolsParams) (*mcp.ListToolsResult, error)
	CallTool(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error)
}
type Provider struct {
	session                              Session
	brandTool, scheduledTool, createTool string
	style                                string // legacy-info or hosted-native
}

func getProperties(tool *mcp.Tool) map[string]any {
	if tool == nil {
		return nil
	}
	if schema, ok := tool.InputSchema.(map[string]any); ok {
		if props, ok := schema["properties"].(map[string]any); ok {
			return props
		}
	}
	var m map[string]any
	b, _ := json.Marshal(tool.InputSchema)
	_ = json.Unmarshal(b, &m)
	props, _ := m["properties"].(map[string]any)
	return props
}
func hasProps(tool *mcp.Tool, keys ...string) bool {
	props := getProperties(tool)
	if len(props) == 0 {
		return false
	}
	for _, key := range keys {
		if _, ok := props[key]; !ok {
			return false
		}
	}
	return true
}
func NewProvider(ctx context.Context, s Session) (*Provider, error) {
	if s == nil {
		return nil, errors.New("authenticated MCP session required")
	}
	list, err := s.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("Metricool tool discovery failed: %w", err)
	}
	if list.NextCursor != "" {
		return nil, errors.New("unreviewed paginated Metricool tool list; refuse connection")
	}
	tools := map[string]*mcp.Tool{}
	for _, t := range list.Tools {
		tools[t.Name] = t
	}
	p := &Provider{session: s}
	// Native ChatGPT-era schema with an entire data object passed as JSON
	// string. It supports explicit draft=true + autoPublish=false.
	if tools["getBrandSettings"] != nil &&
		hasProps(tools["getScheduledPosts"], "brandId", "fromDate", "toDate", "timezone") &&
		hasProps(tools["createScheduledPost"], "date", "blogId", "info") {
		p.brandTool = "getBrandSettings"
		p.scheduledTool = "getScheduledPosts"
		p.createTool = "createScheduledPost"
		p.style = "legacy-info"
		return p, nil
	}
	// Hosted server tool names may use snake_case, but we only enable a
	// private draft when it exposes explicit draft and auto_publish switches.
	// Many public "schedule" APIs don't: in that case FAIL CLOSED.
	if tools["get_brand_settings"] != nil &&
		hasProps(tools["get_scheduled_posts"], "brand_id", "from_date", "to_date", "timezone") &&
		hasProps(tools["create_scheduled_post"], "blog_id", "date", "timezone", "networks", "text", "media", "draft", "auto_publish") {
		p.brandTool = "get_brand_settings"
		p.scheduledTool = "get_scheduled_posts"
		p.createTool = "create_scheduled_post"
		p.style = "hosted-native"
		return p, nil
	}
	return nil, errors.New("Metricool MCP tools/schema do not prove a private draft-only operation; refuse connection")
}

// ConnectHosted uses the official Metricool MCP endpoint with OAuth provided
// by a trusted local client. The owner must separately consent via a normal
// OAuth browser flow; no login, token or API key is bootstrapped here.
func ConnectHosted(ctx context.Context, handler auth.OAuthHandler) (*Provider, *mcp.ClientSession, error) {
	if handler == nil {
		return nil, nil, errors.New("owner-authorized OAuth handler required")
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "balkazar-gigops-draft-bridge", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: hostedMetricoolMCP, OAuthHandler: handler, MaxRetries: -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		return nil, nil, err
	}
	provider, err := NewProvider(ctx, session)
	if err != nil {
		_ = session.Close()
		return nil, nil, err
	}
	return provider, session, nil
}

func (p *Provider) call(ctx context.Context, name string, args any) ([]byte, error) {
	if p == nil || p.session == nil {
		return nil, errors.New("Metricool MCP provider not configured")
	}
	if name != p.brandTool && name != p.scheduledTool && name != p.createTool {
		return nil, errors.New("refusing non-allowlisted Metricool MCP tool")
	}
	r, err := p.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	if r == nil || r.IsError {
		return nil, errors.New("Metricool MCP returned an error")
	}
	if r.StructuredContent != nil {
		b, err := json.Marshal(r.StructuredContent)
		if err != nil {
			return nil, err
		}
		if len(b) > maxToolResultBytes {
			return nil, errors.New("Metricool result exceeds size limit")
		}
		return b, nil
	}
	for _, content := range r.Content {
		textContent, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(textContent.Text)
		if len(trimmed) > maxToolResultBytes {
			return nil, errors.New("Metricool result exceeds size limit")
		}
		if json.Valid([]byte(trimmed)) {
			return []byte(trimmed), nil
		}
	}
	return nil, errors.New("Metricool MCP returned no structured JSON receipt")
}
func (p *Provider) Brand(ctx context.Context) (gigops.ConnectedMetricoolBrand, error) {
	raw, err := p.call(ctx, p.brandTool, map[string]any{})
	if err != nil {
		return gigops.ConnectedMetricoolBrand{}, err
	}
	var data struct {
		Data []struct {
			ID           json.Number    `json:"id"`
			Label        string         `json:"label"`
			Timezone     string         `json:"timezone"`
			NetworksData map[string]any `json:"networksData"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return gigops.ConnectedMetricoolBrand{}, fmt.Errorf("unrecognized brand response: %w", err)
	}
	for _, v := range data.Data {
		if v.ID.String() != "7273414" {
			continue
		}
		networks := []string{}
		for _, item := range []struct{ field, network string }{
			{"facebookData", "facebook"}, {"instagramData", "instagram"}, {"threadsData", "threads"},
		} {
			if raw, ok := v.NetworksData[item.field]; ok && raw != nil && fmt.Sprint(raw) != "" {
				networks = append(networks, item.network)
			}
		}
		return gigops.ConnectedMetricoolBrand{
			ID: v.ID.String(), Name: v.Label, Timezone: v.Timezone, Networks: networks,
		}, nil
	}
	return gigops.ConnectedMetricoolBrand{}, errors.New("Balkazar Metricool brand missing in authenticated account")
}
func (p *Provider) ScheduledPosts(ctx context.Context, brandID string, from, to time.Time) ([]byte, error) {
	if brandID != "7273414" || to.Before(from) {
		return nil, errors.New("invalid brand or date range")
	}
	var args any
	if p.style == "legacy-info" {
		args = map[string]any{"brandId": brandID, "fromDate": from.Format(time.RFC3339),
			"toDate": to.Format(time.RFC3339), "timezone": "Asia/Taipei", "extendedRange": false}
	} else {
		args = map[string]any{"brand_id": brandID, "from_date": from.Format(time.RFC3339),
			"to_date": to.Format(time.RFC3339), "timezone": "Asia/Taipei", "extended_range": false}
	}
	raw, err := p.call(ctx, p.scheduledTool, args)
	if err != nil {
		return nil, err
	}
	// The core parser accepts only a bounded JSON object with data:[...].
	var check struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &check); err != nil || check.Data == nil {
		return nil, errors.New("Metricool planner did not return a data array")
	}
	return raw, nil
}

func (p *Provider) CreatePrivateDraft(ctx context.Context, args gigops.MetricoolDraftArguments) (gigops.MetricoolDraftReceipt, error) {
	if args.BlogID != "7273414" || p.createTool == "" {
		return gigops.MetricoolDraftReceipt{}, errors.New("private draft tool missing or wrong brand")
	}
	// Revalidate all critical flags at the LAST possible step before the
	// remote MCP call. Never pass arbitrary model-built info to a write tool.
	var payload struct {
		Draft       bool     `json:"draft"`
		AutoPublish bool     `json:"autoPublish"`
		Text        string   `json:"text"`
		Media       []string `json:"media"`
		Providers   []struct {
			Network string `json:"network"`
		} `json:"providers"`
	}
	if err := json.Unmarshal([]byte(args.Info), &payload); err != nil {
		return gigops.MetricoolDraftReceipt{}, err
	}
	if !payload.Draft || payload.AutoPublish || len(payload.Text) < 35 || len(payload.Providers) == 0 {
		return gigops.MetricoolDraftReceipt{}, errors.New("refusing non-draft or incomplete provider payload")
	}
	var postArgs any
	switch p.style {
	case "legacy-info":
		postArgs = map[string]any{"date": args.Date, "blogId": args.BlogID, "info": args.Info}
	case "hosted-native":
		networks := []string{}
		for _, n := range payload.Providers {
			networks = append(networks, n.Network)
		}
		postArgs = map[string]any{
			"blog_id": args.BlogID, "date": args.Date, "timezone": "Asia/Taipei",
			"networks": networks, "text": payload.Text, "media": payload.Media,
			"draft": true, "auto_publish": false,
		}
	default:
		return gigops.MetricoolDraftReceipt{}, errors.New("unrecognized MCP write schema")
	}
	raw, err := p.call(ctx, p.createTool, postArgs)
	if err != nil {
		return gigops.MetricoolDraftReceipt{}, err
	}
	// No speculative "success". If the connector omits stable IDs, the core
	// records an ambiguous attempt and reconciles later via fresh readback.
	var envelope struct {
		Data struct {
			ID   json.Number `json:"id"`
			UUID string      `json:"uuid"`
		} `json:"data"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&envelope); err != nil {
		return gigops.MetricoolDraftReceipt{}, err
	}
	if envelope.Data.ID.String() == "" || envelope.Data.UUID == "" {
		return gigops.MetricoolDraftReceipt{}, errors.New("Metricool did not return stable planner ID/UUID; outcome uncertain")
	}
	return gigops.MetricoolDraftReceipt{UUID: envelope.Data.UUID, PostID: envelope.Data.ID.String()}, nil
}

// ConnectViaPinnedProxy uses a pre-installed, owner-reviewed stdio-to-remote
// OAuth proxy. No npm install, shell interpolation, unknown MCP endpoint, or
// token export occurs here. Run ONLY under the separately permissioned bridge
// OS identity; its OAuth token cache must be inaccessible to the AI agent.
func ConnectViaPinnedProxy(ctx context.Context, proxyExecutable string) (*Provider, *mcp.ClientSession, error) {
	if !filepath.IsAbs(proxyExecutable) {
		return nil, nil, errors.New("trusted proxy executable path must be absolute")
	}
	st, err := os.Stat(proxyExecutable)
	if err != nil {
		return nil, nil, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
		return nil, nil, errors.New("proxy executable is not a regular protected file")
	}
	cmd := exec.Command(proxyExecutable, hostedMetricoolMCP)
	transport := &mcp.CommandTransport{Command: cmd}
	client := mcp.NewClient(&mcp.Implementation{Name: "balkazar-musicops-trusted-bridge", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, err
	}
	provider, err := NewProvider(ctx, session)
	if err != nil {
		_ = session.Close()
		return nil, nil, err
	}
	return provider, session, nil
}
