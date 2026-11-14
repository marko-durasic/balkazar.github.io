package main

// A private local operator CLI for the autonomous MusicOps runner.
// No HTTP client, external social publisher, payment or account write exists.
import (
  "crypto/ed25519"
  "encoding/base64"
  "encoding/json"
  "errors"
  "flag"
  "fmt"
  "os"
  "strings"
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

func draftPreflight(args []string)error{
  f:=flag.NewFlagSet("draft-preflight",flag.ContinueOnError)
  storePath:=f.String("store","","Existing private campaign store JSON")
  candidatePath:=f.String("candidate","","Draft candidate JSON with exact creative and source revision")
  grantPath:=f.String("signed-grant","","Signed owner draft-only grant JSON")
  pubkeyPath:=f.String("trusted-key","","Owner-pinned base64 Ed25519 public key file")
  revokedPath:=f.String("revocations","","Trusted operator revocation list (JSON array)")
  usagePath:=f.String("usage","","Private ledger of grant receipts and in-flight intent keys")
  snapshotPath:=f.String("snapshot","","Optional caller-supplied Metricool getScheduledPosts JSON")
  if err:=f.Parse(args);err!=nil{return err}
  for _,p:=range []string{*storePath,*candidatePath,*grantPath,*pubkeyPath,*revokedPath,*usagePath}{
    if p==""{return errors.New("store, candidate, signed-grant, trusted-key, revocations and usage paths are all required")}
  }
  readJSON:=func(path string,target any)error{
    data,err:=os.ReadFile(path);if err!=nil{return err}
    if len(data)>1<<20{return errors.New("read-only input exceeds 1MiB")}
    return json.Unmarshal(data,target)
  }
  saved,err:=gigops.LoadCampaignStore(*storePath);if err!=nil{return err}
  var candidate gigops.MetricoolDraftCandidate
  if err:=readJSON(*candidatePath,&candidate);err!=nil{return err}
  var signed gigops.SignedMetricoolDraftGrant
  if err:=readJSON(*grantPath,&signed);err!=nil{return err}
  rawKey,err:=os.ReadFile(*pubkeyPath);if err!=nil{return err}
  key,err:=base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(rawKey)))
  if err!=nil||len(key)!=ed25519.PublicKeySize{return errors.New("invalid trusted Ed25519 public key")}
  var revocations []string
  if err:=readJSON(*revokedPath,&revocations);err!=nil{return err}
  revoked:=map[string]bool{}
  for _,id:=range revocations{revoked[id]=true}
  var usage gigops.GrantUsage
  if err:=readJSON(*usagePath,&usage);err!=nil{return err}
  trust:=gigops.MetricoolGrantTrust{OwnerPublicKey:ed25519.PublicKey(key),RevokedGrantIDs:revoked}
  var preview *gigops.MetricoolReadbackPreview
  if *snapshotPath!=""{
    campaign,ok:=saved.Campaigns[candidate.GigID]
    if !ok{return errors.New("gig not in private campaign store")}
    snapshot,err:=os.Open(*snapshotPath);if err!=nil{return err}
    defer snapshot.Close()
    p,err:=gigops.PreviewMetricoolSnapshot(snapshot,campaign.Gig,"7273414")
    if err!=nil{return err}
    preview=&p
  }
  result,err:=gigops.BuildMetricoolDraftIntent(saved,candidate,signed,trust,usage,preview,time.Now())
  if err!=nil{return err}
  return encode(result)
}

func run()error{
  if len(os.Args)<2{return errors.New("usage: musicops-ops status|private-job|metricool-readback|draft-preflight [flags]")}
  switch os.Args[1]{
  case "status":return status(os.Args[2:])
  case "private-job":return job(os.Args[2:])
  case "metricool-readback":return metricool(os.Args[2:])
  case "draft-preflight":return draftPreflight(os.Args[2:])
  default:return errors.New("unknown operation; allowed: status, private-job, metricool-readback, draft-preflight")
  }
}
func main(){fail(run())}
