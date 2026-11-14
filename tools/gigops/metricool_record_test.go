package gigops

import (
  "strings"
  "testing"
  "time"
)

func metricoolPreviewFor(t *testing.T,g GigIntake,p ...map[string]any) MetricoolReadbackPreview {
  t.Helper()
  r,err:=PreviewMetricoolSnapshot(strings.NewReader(metricoolFixture(p...)),g,balkazarMetricoolBrandID)
  if err!=nil{t.Fatal(err)}
  return r
}
func TestMetricoolReadbackPersistsMetadataWithoutApproval(t *testing.T){
  store:=NewCampaignStore();g:=sampleGig()
  if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  preview:=metricoolPreviewFor(t,g,metricoolPostFixture("metricool-safe-uuid-1",true))
  result,err:=store.RecordMetricoolReadback(preview,storeTime())
  if err!=nil{t.Fatal(err)}
  if result.PublicWrites!=0||result.VerifiedPublications!=0||result.ObservationChanges!=1{
    t.Fatalf("readback incorrectly authorized/published: %+v",result)
  }
  saved:=store.Campaigns[g.GigID]
  if saved.Jobs["schedule-social-promotion"].State!="approval-required"{
    t.Fatal("readback bypassed social approval")
  }
  if len(saved.MetricoolObservations)!=1||saved.MetricoolObservations[0].PostUUID!="metricool-safe-uuid-1"{
    t.Fatal("lost readback identity")
  }
  same,err:=store.RecordMetricoolReadback(preview,storeTime().Add(time.Hour))
  if err!=nil||same.ObservationChanges!=0{t.Fatal("unchanged snapshot rewrote receipt")}
}
func TestMetricoolPlannerIDChangeKeepsSameUUID(t *testing.T){
  store:=NewCampaignStore();g:=sampleGig()
  if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  p:=metricoolPostFixture("metricool-stable-uuid",true)
  if _,err:=store.RecordMetricoolReadback(metricoolPreviewFor(t,g,p),storeTime());err!=nil{t.Fatal(err)}
  p["id"]=421679436
  result,err:=store.RecordMetricoolReadback(metricoolPreviewFor(t,g,p),storeTime().Add(time.Hour))
  if err!=nil{t.Fatal(err)}
  saved:=store.Campaigns[g.GigID].MetricoolObservations
  if result.ObservationChanges!=1||len(saved)!=1||saved[0].PlannerPostID!="421679436"||len(saved[0].Previous)!=1{
    t.Fatalf("planner edit duplicated rather than revised post: %+v",saved)
  }
}
func TestMetricoolReadbackSurvivesRescheduleAndCancellation(t *testing.T){
  store:=NewCampaignStore();g:=sampleGig()
  if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  if _,err:=store.RecordMetricoolReadback(metricoolPreviewFor(t,g,metricoolPostFixture("safe-post-111",true)),storeTime());err!=nil{t.Fatal(err)}
  g.Start="2026-12-04T20:00:00+08:00";g.End="2026-12-04T21:30:00+08:00"
  revised,err:=store.ReconcileGig(g,storeTime().Add(time.Hour))
  if err!=nil{t.Fatal(err)}
  if len(revised.Campaign.MetricoolObservations)!=1||
    revised.Campaign.MetricoolObservations[0].SourceRevision!=1||
    revised.Campaign.Jobs["schedule-social-promotion"].State!="reconciliation-required"{
    t.Fatal("reschedule failed to preserve old draft for reconciliation")
  }
  g.BookingStatus="cancelled"
  cancelled,err:=store.ReconcileGig(g,storeTime().Add(2*time.Hour))
  if err!=nil{t.Fatal(err)}
  if len(cancelled.Campaign.MetricoolObservations)!=1||len(cancelled.Campaign.Jobs)!=2{
    t.Fatal("cancellation discarded historical planner draft or left new promotion")
  }
}
func TestMetricoolSnapshotCannotBeLinkedToDifferentGig(t *testing.T){
  s:=NewCampaignStore();one:=sampleGig()
  if _,err:=s.ReconcileGig(one,storeTime());err!=nil{t.Fatal(err)}
  two:=one
  two.GigID="different-balkazar-gig"
  two.OfficialSourceURL="https://balkazar.org/events/2026-12-11-other-gig/"
  two.Start="2026-12-11T20:00:00+08:00";two.End="2026-12-11T21:00:00+08:00"
  if _,err:=s.ReconcileGig(two,storeTime());err!=nil{t.Fatal(err)}
  originalPost:=metricoolPostFixture("cross-gig-uuid-1",true)
  if _,err:=s.RecordMetricoolReadback(metricoolPreviewFor(t,one,originalPost),storeTime());err!=nil{t.Fatal(err)}
  originalPost["text"]="Look: "+two.OfficialSourceURL
  _,err:=s.RecordMetricoolReadback(metricoolPreviewFor(t,two,originalPost),storeTime())
  if err==nil||!strings.Contains(err.Error(),"already recorded"){t.Fatalf("cross-gig uuid accepted: %v",err)}
  wrongReport:=metricoolPreviewFor(t,one,metricoolPostFixture("another-uuid-2",true))
  wrongReport.GigID=two.GigID
  if _,err:=s.RecordMetricoolReadback(wrongReport,storeTime());err==nil{t.Fatal("matched wrong gig canonical URL")}
}
func TestMetricoolReadbackPrivateDiskRoundtrip(t *testing.T){
  path:=privateStorePath(t);g:=sampleGig()
  if _,err:=ApplyGigIntakeFile(path,g,storeTime());err!=nil{t.Fatal(err)}
  snapshot:=metricoolFixture(metricoolPostFixture("metricool-real-style-1",true))
  result,err:=ApplyMetricoolReadbackFile(path,g.GigID,balkazarMetricoolBrandID,strings.NewReader(snapshot),storeTime())
  if err!=nil{t.Fatal(err)}
  if !result.LocalStateWritten||result.PublicWrites!=0||result.VerifiedPublications!=0{
    t.Fatal("unexpected disk receipt state")
  }
  again,err:=ApplyMetricoolReadbackFile(path,g.GigID,balkazarMetricoolBrandID,strings.NewReader(snapshot),storeTime().Add(time.Hour))
  if err!=nil||again.ObservationChanges!=0||again.LocalStateWritten{
    t.Fatal("duplicate readback wrote state")
  }
  onDisk,err:=LoadCampaignStore(path);if err!=nil{t.Fatal(err)}
  if len(onDisk.Campaigns[g.GigID].MetricoolObservations)!=1{t.Fatal("lost observations")}
}
