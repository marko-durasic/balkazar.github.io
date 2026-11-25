package gigops

import (
  "encoding/json"
  "strings"
  "testing"
)

func metricoolFixture(posts ...map[string]any) string {
  payload,_:=json.Marshal(map[string]any{"data":posts})
  return string(payload)
}
func metricoolPostFixture(uuid string,draft bool)map[string]any{
  return map[string]any{
    "id":421679435, "uuid":uuid, "draft":draft, "autoPublish":false,
    "text":"See you: "+sampleGig().OfficialSourceURL,
    "media":[]string{"https://example.org/photo.jpg"},
    "publicationDate":map[string]any{"dateTime":"2026-11-24T19:00:00","timezone":"Asia/Taipei"},
    "providers":[]map[string]string{
      {"network":"instagram","status":"PENDING"},
      {"network":"facebook","status":"PENDING"},
    },
  }
}
func TestMetricoolReadbackRecognizesRealShapeWithoutPublishing(t *testing.T){
  input:=metricoolFixture(metricoolPostFixture("6368837845729689867",true))
  r,err:=PreviewMetricoolSnapshot(strings.NewReader(input),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if !r.ReadOnly||r.PublicWrites!=0||len(r.MatchedPosts)!=1{t.Fatalf("incorrect readback: %+v",r)}
  p:=r.MatchedPosts[0]
  if p.State!="observed-draft"||p.AutoPublish||p.PostUUID!="6368837845729689867"||p.PlannerPostID!="421679435"{
     t.Fatalf("misclassified real draft: %+v",p)
  }
  if p.ContentSHA256==""||p.PublicationAt!="2026-11-24T19:00:00+08:00"{
     t.Fatalf("missing readback metadata: %+v",p)
  }
}
func TestMetricoolReadbackNeverInfersPublicPosting(t *testing.T){
  queued:=metricoolPostFixture("balkazar-queued-1",false)
  r,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(queued)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if r.MatchedPosts[0].State!="observed-queued"{t.Fatal("queue not detected")}
  queued["providers"]=[]map[string]string{{"network":"instagram","status":"PUBLISHED"}}
  r,err=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(queued)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if r.MatchedPosts[0].State!="needs-review"||len(r.Warnings)==0{
      t.Fatal("scheduled-only readback falsely treated as published")
  }
}
func TestMetricoolReadbackIgnoresUnrelatedAndSpoofedLinks(t *testing.T){
  p:=metricoolPostFixture("balkazar-safe-1",true)
  p["text"]="go to https://balkazar.org.evil.example/events/2026-12-03-european-christmas-market/"
  r,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(p)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if len(r.MatchedPosts)!=0||r.OtherPostCount!=1{t.Fatal("accepted spoofed link")}
  p["text"]="https://balkazar.org/events/2026-12-03-european-christmas-market/evil"
  r,err=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(p)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if len(r.MatchedPosts)!=0{t.Fatal("accepted longer unrelated URL")}
}
func TestMetricoolReadbackPreservesStableUUIDOnPostIDEdit(t *testing.T){
  p:=metricoolPostFixture("balkazar-stable-uuid",true)
  first,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(p)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  p["id"]=421679436
  next,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(p)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if first.MatchedPosts[0].PostUUID!=next.MatchedPosts[0].PostUUID||first.MatchedPosts[0].PlannerPostID==next.MatchedPosts[0].PlannerPostID{
    t.Fatal("failed UUID identity on edited planner post")
  }
}
func TestMetricoolReadbackRejectsWrongTenantAndBrokenData(t *testing.T){
  p:=metricoolPostFixture("balkazar-safe-1",true)
  good:=metricoolFixture(p)
  g:=sampleGig()
  if _,err:=PreviewMetricoolSnapshot(strings.NewReader(good),g,"7273415");err==nil{t.Fatal("wrong Metricool brand accepted")}
  g.Artist="Fuzzy Wall"
  if _,err:=PreviewMetricoolSnapshot(strings.NewReader(good),g,balkazarMetricoolBrandID);err==nil{t.Fatal("cross-tenant accepted")}
  for _,s:=range []string{"{}","{\"data\":null}","{\"data\":[{}]}","{\"data\":[",good+"{\"extra\":true}"}{
    if _,err:=PreviewMetricoolSnapshot(strings.NewReader(s),sampleGig(),balkazarMetricoolBrandID);err==nil{
      t.Fatalf("accepted malformed or ambiguous snapshot %s",s)
    }
  }
}
func TestMetricoolReadbackRejectsDuplicateIDsAndTimezoneErrors(t *testing.T){
  a:=metricoolPostFixture("duplicate-uuid",true)
  if _,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(a,a)),sampleGig(),balkazarMetricoolBrandID);err==nil{
    t.Fatal("duplicate same post UUID accepted")
  }
  a["publicationDate"]=map[string]string{"dateTime":"2026-11-24T19:00:00","timezone":"Europe/Madrid"}
  if _,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(a)),sampleGig(),balkazarMetricoolBrandID);err==nil{
    t.Fatal("wrong timezone accepted")
  }
}
func TestMetricoolReadbackDetectsDuplicateCreative(t *testing.T){
  first:=metricoolPostFixture("balkazar-post-100",true)
  second:=metricoolPostFixture("balkazar-post-101",true)
  second["id"]=421679439
  r,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(first,second)),sampleGig(),balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  if len(r.MatchedPosts)!=2||len(r.Warnings)==0{t.Fatal("duplicate creative missed")}
}
