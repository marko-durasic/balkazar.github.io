package main

// Local-only read-only MCP plugin. No TCP listener, browser control,
// account credentials, generic file reads, or publication operations.
import (
 "context"
 "crypto/ed25519"
 "encoding/base64"
 "encoding/json"
 "strings"
 "fmt"
 "log"
 "os"
 "time"
 gigops "balkazar.org/musicops"
 "github.com/modelcontextprotocol/go-sdk/mcp"
)
type EmptyInput struct{}
type SourceInput struct { SourceURL string `json:"source_url" jsonschema:"Exact approved Balkazar gig page URL"` }
type PlansOutput struct { Events []gigops.EventPlan `json:"events"` }
type PlanOutput struct { Event gigops.EventPlan `json:"event"` }

func getPlans() ([]gigops.EventPlan,error){
 path:=os.Getenv("MUSICOPS_REVIEW_BUNDLE")
 if path=="" { return nil,fmt.Errorf("MUSICOPS_REVIEW_BUNDLE must point to a local approved bundle") }
 b,err:=gigops.LoadReviewBundle(path);if err!=nil{return nil,err}
 ledgerPath:=os.Getenv("MUSICOPS_LEDGER")
 if ledgerPath=="" { ledgerPath,err=gigops.DefaultLedgerPath();if err!=nil{return nil,err} }
 ledger,err:=gigops.LoadLedger(ledgerPath);if err!=nil{return nil,err}
 return gigops.Plans(b,ledger),nil
}
func listPlans(_ context.Context,_ *mcp.CallToolRequest,_ EmptyInput)(*mcp.CallToolResult,PlansOutput,error){
 plans,err:=getPlans();if err!=nil{return nil,PlansOutput{},err};return nil,PlansOutput{Events:plans},nil
}
func getPlan(_ context.Context,_ *mcp.CallToolRequest,input SourceInput)(*mcp.CallToolResult,PlanOutput,error){
 if !gigops.ValidSourceURL(input.SourceURL){return nil,PlanOutput{},fmt.Errorf("unapproved source URL")}
 plans,err:=getPlans();if err!=nil{return nil,PlanOutput{},err}
 for _,p:=range plans {if p.SourceURL==input.SourceURL{return nil,PlanOutput{Event:p},nil}}
 return nil,PlanOutput{},fmt.Errorf("source event not found in reviewed bundle")
}
type IntakeOutput struct { Campaign gigops.GigIntakePlan `json:"campaign"` }
func previewGigIntake(_ context.Context,_ *mcp.CallToolRequest,input gigops.GigIntake)(*mcp.CallToolResult,IntakeOutput,error){
 plan,err:=gigops.PlanGigIntake(input,time.Now());if err!=nil{return nil,IntakeOutput{},err}
 return nil,IntakeOutput{Campaign:plan},nil
}

// Draft preflight reads ONLY explicitly configured fixed local files. None of
// their paths are supplied by MCP tool input, and no external post is created.
type DraftOutput struct { Intent gigops.MetricoolDraftIntent `json:"intent"` }
func requireLocalEnv(name string)(string,error){
  v:=os.Getenv(name)
  if v==""{return "",fmt.Errorf("%s must be pinned in the trusted local MCP process",name)}
  return v,nil
}
func readSmallJSON(path string,target any)error{
  b,err:=os.ReadFile(path);if err!=nil{return err}
  if len(b)>1<<20{return fmt.Errorf("local policy file exceeds 1 MiB")}
  return json.Unmarshal(b,target)
}
func previewMetricoolDraft(_ context.Context,_ *mcp.CallToolRequest,input gigops.MetricoolDraftCandidate)(*mcp.CallToolResult,DraftOutput,error){
  path,err:=requireLocalEnv("MUSICOPS_CAMPAIGN_STORE")
  if err!=nil{return nil,DraftOutput{},err}
  store,err:=gigops.LoadCampaignStore(path)
  if err!=nil{return nil,DraftOutput{},err}
  grantPath,err:=requireLocalEnv("MUSICOPS_DRAFT_SIGNED_GRANT")
  if err!=nil{return nil,DraftOutput{},err}
  var signed gigops.SignedMetricoolDraftGrant
  if err:=readSmallJSON(grantPath,&signed);err!=nil{return nil,DraftOutput{},err}
  keyPath,err:=requireLocalEnv("MUSICOPS_OWNER_PINNED_PUBKEY")
  if err!=nil{return nil,DraftOutput{},err}
  kb,err:=os.ReadFile(keyPath)
  if err!=nil{return nil,DraftOutput{},err}
  key,err:=base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(kb)))
  if err!=nil||len(key)!=ed25519.PublicKeySize{return nil,DraftOutput{},fmt.Errorf("invalid pinned Ed25519 owner public key")}
  revokedPath,err:=requireLocalEnv("MUSICOPS_DRAFT_REVOCATIONS")
  if err!=nil{return nil,DraftOutput{},err}
  var revokedIDs []string
  if err:=readSmallJSON(revokedPath,&revokedIDs);err!=nil{return nil,DraftOutput{},err}
  revoked:=map[string]bool{}
  for _,id:=range revokedIDs{revoked[id]=true}
  usagePath,err:=requireLocalEnv("MUSICOPS_DRAFT_USAGE")
  if err!=nil{return nil,DraftOutput{},err}
  var usage gigops.GrantUsage
  if err:=readSmallJSON(usagePath,&usage);err!=nil{return nil,DraftOutput{},err}
  trust:=gigops.MetricoolGrantTrust{OwnerPublicKey:ed25519.PublicKey(key),RevokedGrantIDs:revoked}
  // A trusted execution service will eventually supply direct fresh provider
  // results. The read-only local MCP does not accept a forged snapshot as a
  // substitute and therefore always returns execution_allowed=false.
  intent,err:=gigops.BuildMetricoolDraftIntent(store,input,signed,trust,usage,nil,time.Now())
  if err!=nil{return nil,DraftOutput{},err}
  return nil,DraftOutput{Intent:intent},nil
}

func main(){
 s:=mcp.NewServer(&mcp.Implementation{Name:"balkazar-musicops",Version:"0.1.0"},nil)
 mcp.AddTool(s,&mcp.Tool{Name:"musicops_list_gig_plans",Description:"Read-only local gig listing plans and blockers; never contacts or publishes on Songkick."},listPlans)
 mcp.AddTool(s,&mcp.Tool{Name:"musicops_get_gig_plan",Description:"Read-only plan for a single approved Balkazar event URL; no browser actions."},getPlan)
 mcp.AddTool(s,&mcp.Tool{Name:"musicops_preview_gig_intake",Description:"Read-only gig intake. Produce candidate channels, blockers and a no-public-write campaign workflow from structured, confirmed gig facts. Does not create listings, schedule posts or contact accounts."},previewGigIntake)
 mcp.AddTool(s,&mcp.Tool{Name:"musicops_preview_metricool_draft",Description:"READ-ONLY and non-executable: validate a gig-specific draft candidate using operator-pinned owner-signed grant, revocation and usage files; return connector arguments for review, never call Metricool or publish."},previewMetricoolDraft)
 if err:=s.Run(context.Background(),&mcp.StdioTransport{});err!=nil{log.Fatal(err)}
}
