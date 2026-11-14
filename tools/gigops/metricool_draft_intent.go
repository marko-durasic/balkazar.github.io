package gigops

// BuildMetricoolDraftIntent is a pure, non-publishing provider adapter.
// A trusted connector runner must fetch fresh provider state immediately before
// executing ANY external call; caller-supplied snapshots cannot authorize it.
import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "sort"
 "strings"
 "time"
)

type MetricoolDraftCandidate struct {
 GigID string `json:"gig_id"`
 SourceRevision uint64 `json:"source_revision"`
 PublicationAt string `json:"publication_at"` // RFC3339
 Networks []string `json:"networks"`
 Text string `json:"text"`
 Media []string `json:"media"`
}
type MetricoolDraftIntent struct {
 Status string `json:"status"` // existing-draft, blocked-conflict, needs-live-readback
 ReadOnly bool `json:"read_only"`
 PublicWrites int `json:"public_writes"`
 ExecutionAllowed bool `json:"execution_allowed"`
 BrandID string `json:"brand_id"`
 GigID string `json:"gig_id"`
 SourceRevision uint64 `json:"source_revision"`
 IntentKey string `json:"intent_key"`
 ExistingUUID string `json:"existing_uuid,omitempty"`
 Blocker string `json:"blocker,omitempty"`
 // Only consume these args after trusted runtime performs a new direct provider
 // preflight and atomically reserves the intent key in its private ledger.
 CreatePostArguments *MetricoolDraftArguments `json:"create_post_arguments,omitempty"`
}
type MetricoolDraftArguments struct {
 BlogID string `json:"blog_id"`
 Date string `json:"date"`
 Info string `json:"info"` // matches createScheduledPost connector's "info" JSON string
}

func sameStringList(a,b []string)bool {
 if len(a)!=len(b){return false}
 for i:=range a{if a[i]!=b[i]{return false}}
 return true
}
func contentDigest(text string,media,networks []string)string {
 b,_:=json.Marshal([]any{text,media,networks})
 sum:=sha256.Sum256(b)
 return hex.EncodeToString(sum[:])
}
func canonicalNetworks(networks []string)([]string,error){
 if !validStringSet(networks,3,isDraftNetwork){
   return nil,errors.New("draft networks must be unique Facebook, Instagram or Threads")
 }
 sorted:=append([]string(nil),networks...)
 sort.Strings(sorted)
 return sorted,nil
}
func preflightDraftData(c StoredCampaign, draft MetricoolDraftCandidate, now time.Time) ([]string,time.Time,string,error) {
 if c.Gig.BookingStatus!="confirmed" {
   return nil,time.Time{},"",errors.New("cannot draft for unconfirmed or cancelled gig")
 }
 if c.Revision!=draft.SourceRevision||c.Gig.GigID!=draft.GigID||draft.SourceRevision==0{
   return nil,time.Time{},"",errors.New("stale source revision or incorrect gig ID")
 }
 if !ValidSourceURL(c.Gig.OfficialSourceURL)||!exactEventLinkInText(draft.Text,c.Gig.OfficialSourceURL){
   return nil,time.Time{},"",errors.New("draft text must contain exact canonical gig URL")
 }
 if len(draft.Text)<35||len(draft.Text)>3000||strings.ContainsRune(draft.Text,0){
   return nil,time.Time{},"",errors.New("draft body is empty/oversize/invalid")
 }
 if len(draft.Media)>4{return nil,time.Time{},"",errors.New("too many draft attachments")}
 for _,m:=range draft.Media{
   if !validGrantMediaURL(m){return nil,time.Time{},"",errors.New("draft media not from approved owned assets")}
 }
 networks,err:=canonicalNetworks(draft.Networks);if err!=nil{return nil,time.Time{},"",err}
 for _,network:=range networks{
   if network=="instagram"&&len(draft.Media)==0 {
     return nil,time.Time{},"",errors.New("Instagram post requires an approved image/video asset")
   }
 }
 when,err:=time.Parse(time.RFC3339,draft.PublicationAt)
 if err!=nil||!when.After(now)||when.Sub(now)>90*24*time.Hour {
   return nil,time.Time{},"",errors.New("draft schedule must be future and within 90 days")
 }
 if c.Gig.Timezone!="Asia/Taipei"{return nil,time.Time{},"",errors.New("Balkazar pilot requires Asia/Taipei")}
 _,off:=when.Zone()
 _,expected:=when.In(time.FixedZone("TW",8*3600)).Zone()
 if off!=expected{return nil,time.Time{},"",errors.New("draft schedule offset not Taipei")}
 showTime,err:=time.Parse(time.RFC3339,c.Gig.Start)
 if err!=nil||!when.Before(showTime) {return nil,time.Time{},"",errors.New("pre-show draft must be before the performance")}
 digest:=contentDigest(draft.Text,draft.Media,networks)
 return networks,when,digest,nil
}

// BuildMetricoolDraftIntent provides connector-ready, draft-only arguments.
// It never calls the plugin; ExecutionAllowed is ALWAYS false here because a
// caller-supplied Metricool snapshot is not trustworthy fresh provider proof.
func BuildMetricoolDraftIntent(store CampaignStore,draft MetricoolDraftCandidate,
 signed SignedMetricoolDraftGrant,trust MetricoolGrantTrust,usage GrantUsage,
 preview *MetricoolReadbackPreview,now time.Time)(MetricoolDraftIntent,error){
 c,ok:=store.Campaigns[draft.GigID]
 if !ok{return MetricoolDraftIntent{},errors.New("gig not found in private campaign store")}
 networks,when,creativeSHA,err:=preflightDraftData(c,draft,now)
 if err!=nil{return MetricoolDraftIntent{},err}
 grant,err:=VerifyMetricoolDraftGrant(signed,trust,now)
 if err!=nil{return MetricoolDraftIntent{},err}
 if grant.GigID!="*"&&grant.GigID!=draft.GigID{
   return MetricoolDraftIntent{},errors.New("signed draft grant excludes this gig")
 }
 for _,network:=range networks{
   if !allowed(network,grant.Networks...){return MetricoolDraftIntent{},fmt.Errorf("network %s not covered by signed grant",network)}
 }
 for _,m:=range draft.Media{
   if !allowed(m,grant.MediaURLAllowlist...){return MetricoolDraftIntent{},errors.New("draft media not included in signed rights-reviewed allowlist")}
 }
 keyBytes,_:=json.Marshal([]any{grant.GrantID,draft.GigID,draft.SourceRevision,when.Format(time.RFC3339),networks,creativeSHA})
 digest:=sha256.Sum256(keyBytes)
 key:=hex.EncodeToString(digest[:])
 if err:=usage.MayReserve(grant,key);err!=nil{return MetricoolDraftIntent{},err}
 result:=MetricoolDraftIntent{
   Status:"needs-live-readback",ReadOnly:true,PublicWrites:0,ExecutionAllowed:false,
   BrandID:balkazarMetricoolBrandID,GigID:draft.GigID,SourceRevision:draft.SourceRevision,IntentKey:key,
   Blocker:"Connected trusted runner must fetch current Metricool brand/posts and revalidate grant, dedupe and revision before any draft creation",
 }
 // Existing stored observations are advisory: they can block a duplicate, but
 // missing observations do NOT allow write execution.
 for _,old:=range c.MetricoolObservations {
   if old.ContentSHA256==creativeSHA &&
      old.PublicationAt==when.Format(time.RFC3339) &&
      sameStringList(old.Networks,networks) {
     result.Status="existing-draft"
     result.ExistingUUID=old.PostUUID
     result.Blocker="Existing planner UUID in private history; verify directly with Metricool before reusing or changing it"
     return result,nil
   }
 }
 if preview!=nil {
   if !preview.ReadOnly||preview.PublicWrites!=0||
      preview.BrandID!=balkazarMetricoolBrandID||
      preview.GigID!=draft.GigID||preview.EventURL!=c.Gig.OfficialSourceURL {
     return MetricoolDraftIntent{},errors.New("snapshot does not match gig and brand")
   }
   for _,post:=range preview.MatchedPosts {
     if post.ContentSHA256==creativeSHA &&
        post.PublicationAt==when.Format(time.RFC3339) &&
        sameStringList(post.Networks,networks) {
       result.Status="existing-draft"
       result.ExistingUUID=post.PostUUID
       result.Blocker="Existing UUID observed in caller-supplied planner snapshot; verify live rather than create again"
       return result,nil
     }
     if post.PublicationAt==when.Format(time.RFC3339){
       for _,network:=range networks{
         if allowed(network,post.Networks...) {
           result.Status="blocked-conflict"
           result.Blocker="Another post for this gig and network is planned at the same time; reconcile before creating new draft"
           return result,nil
         }
       }
     }
   }
 }
 // The official Metricool connector uses a JSON-encoded info object.
 info:=map[string]any{
   "autoPublish":false,"draft":true,"text":draft.Text,
   "media":draft.Media,"providers":[]map[string]string{},
   "publicationDate":map[string]string{
     "dateTime":when.In(time.FixedZone("TW",8*3600)).Format("2006-01-02T15:04:05"),
     "timezone":"Asia/Taipei",
   },
 }
 providers:=make([]map[string]string,0,len(networks))
 for _,n:=range networks{
   providers=append(providers,map[string]string{"network":n})
   switch n{
   case "facebook": info["facebookData"]=map[string]string{"type":"POST"}
   case "instagram":info["instagramData"]=map[string]string{"type":"POST"}
   case "threads":info["threadsData"]=map[string]string{"type":"POST"}
   }
 }
 info["providers"]=providers
 blob,err:=json.Marshal(info)
 if err!=nil{return MetricoolDraftIntent{},fmt.Errorf("encode Metricool draft payload: %w",err)}
 result.CreatePostArguments=&MetricoolDraftArguments{
    BlogID:balkazarMetricoolBrandID,Date:when.Format(time.RFC3339),Info:string(blob),
 }
 return result,nil
}
