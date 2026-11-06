package main

// Local-only read-only MCP plugin. No TCP listener, browser control,
// account credentials, generic file reads, or publication operations.
import (
 "context"
 "fmt"
 "log"
 "os"
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
func main(){
 s:=mcp.NewServer(&mcp.Implementation{Name:"balkazar-musicops",Version:"0.1.0"},nil)
 mcp.AddTool(s,&mcp.Tool{Name:"musicops_list_gig_plans",Description:"Read-only local gig listing plans and blockers; never contacts or publishes on Songkick."},listPlans)
 mcp.AddTool(s,&mcp.Tool{Name:"musicops_get_gig_plan",Description:"Read-only plan for a single approved Balkazar event URL; no browser actions."},getPlan)
 if err:=s.Run(context.Background(),&mcp.StdioTransport{});err!=nil{log.Fatal(err)}
}
