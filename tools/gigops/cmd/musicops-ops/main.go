package main

// A private local operator CLI for the autonomous MusicOps runner.
// No HTTP client, external social publisher, payment or account write exists.
import (
  "encoding/json"
  "errors"
  "flag"
  "fmt"
  "os"
  "time"
  gigops "balkazar.org/musicops"
)
func fail(err error){
  if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(2)}
}
func encode(v any)error{
  e:=json.NewEncoder(os.Stdout);e.SetIndent("","  ");return e.Encode(v)
}
func metricool(args []string)error{
  f:=flag.NewFlagSet("metricool-readback",flag.ContinueOnError)
  store:=f.String("store","","Private absolute campaign store .json path")
  gig:=f.String("gig-id","","Existing gig_id")
  brand:=f.String("brand-id","7273414","Metricool brand ID, Balkazar only")
  snapshot:=f.String("snapshot","","Raw JSON data array from connected Metricool getScheduledPosts")
  record:=f.Bool("record",false,"Persist ONLY private ID/status/hash observations; no posts are created")
  if err:=f.Parse(args);err!=nil{return err}
  if *store==""||*gig==""||*snapshot==""{return errors.New("--store, --gig-id and --snapshot are required")}
  file,err:=os.Open(*snapshot);if err!=nil{return err}
  defer file.Close()
  if *record{
    result,err:=gigops.ApplyMetricoolReadbackFile(*store,*gig,*brand,file,time.Now())
    if err!=nil{return err}
    return encode(result)
  }
  saved,err:=gigops.LoadCampaignStore(*store);if err!=nil{return err}
  campaign,ok:=saved.Campaigns[*gig]
  if !ok{return errors.New("gig not in campaign store")}
  report,err:=gigops.PreviewMetricoolSnapshot(file,campaign.Gig,*brand)
  if err!=nil{return err}
  return encode(report)
}
func job(args []string)error{
  f:=flag.NewFlagSet("private-job",flag.ContinueOnError)
  store:=f.String("store","","Private absolute campaign store .json path")
  gig:=f.String("gig-id","","Existing gig_id")
  id:=f.String("job-id","","Internal job ID (not external or physical)")
  revision:=f.Uint64("revision",0,"Expected source revision")
  action:=f.String("action","","start or complete")
  artifact:=f.String("artifact-sha256","","Private artifact SHA256, needed for complete")
  if err:=f.Parse(args);err!=nil{return err}
  if *store==""||*gig==""||*id==""||*revision==0{return errors.New("--store, --gig-id, --job-id and --revision required")}
  result,err:=gigops.ApplyPrivateJobTransitionFile(*store,*gig,*id,*revision,*action,*artifact,time.Now())
  if err!=nil{return err}
  return encode(result)
}
func status(args []string)error{
  f:=flag.NewFlagSet("status",flag.ContinueOnError)
  store:=f.String("store","","Private campaign store .json path")
  gig:=f.String("gig-id","","Existing gig_id")
  if err:=f.Parse(args);err!=nil{return err}
  if *store==""||*gig==""{return errors.New("--store and --gig-id required")}
  saved,err:=gigops.LoadCampaignStore(*store);if err!=nil{return err}
  campaign,ok:=saved.Campaigns[*gig];if !ok{return errors.New("gig not found")}
  return encode(campaign)
}
func run()error{
  if len(os.Args)<2{return errors.New("usage: musicops-ops status|private-job|metricool-readback [flags]")}
  switch os.Args[1]{
  case "status":return status(os.Args[2:])
  case "private-job":return job(os.Args[2:])
  case "metricool-readback":return metricool(os.Args[2:])
  default:return errors.New("unknown operation; allowed: status, private-job, metricool-readback")
  }
}
func main(){fail(run())}
