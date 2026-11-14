package gigops

import (
  "strings"
  "testing"
  "time"
)
const privateDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestPrivateJobTransitionDependencyOrderAndIdempotence(t *testing.T) {
  store:=NewCampaignStore()
  g:=sampleGig()
  if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  if _,err:=store.TransitionPrivateJob(g.GigID,"validate-booking",1,"start","",storeTime());err==nil{
     t.Fatal("accepted downstream job before dependency completion")
  }
  start,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"start","",storeTime())
  if err!=nil{t.Fatal(err)}
  if start.Change!="in-progress"||start.PublicWrites!=0||start.Job.Attempts!=1{t.Fatal("invalid start result")}
  if _,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"complete","bad-hash",storeTime());err==nil{
    t.Fatal("accepted fake/incomplete artifact hash")
  }
  finished,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"complete",privateDigest,storeTime().Add(time.Minute))
  if err!=nil{t.Fatal(err)}
  if finished.Change!="private-complete"||finished.Job.ArtifactSHA256!=privateDigest{t.Fatal("private work not marked complete")}
  oldCount:=len(store.Campaigns[g.GigID].JobHistory)
  again,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"complete",privateDigest,storeTime().Add(time.Hour))
  if err!=nil||again.Change!="unchanged"||len(store.Campaigns[g.GigID].JobHistory)!=oldCount{t.Fatal("non-idempotent private completion")}
  next,err:=store.TransitionPrivateJob(g.GigID,"validate-booking",1,"start","",storeTime())
  if err!=nil||next.Job.State!="in-progress"{t.Fatalf("dependency did not open next job: %v",err)}
  if _,err:=store.TransitionPrivateJob(g.GigID,"schedule-social-promotion",1,"complete",privateDigest,storeTime());err==nil{
    t.Fatal("unapproved external publisher completion accepted")
  }
  if _,err:=store.TransitionPrivateJob(g.GigID,"stage-and-recording",1,"start","",storeTime());err==nil{
    t.Fatal("pretended physical show work is automated")
  }
}
func TestPrivateJobTransitionRequiresMatchingRevision(t *testing.T){
  store:=NewCampaignStore();g:=sampleGig()
  if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  g.Start="2026-12-04T20:00:00+08:00";g.End="2026-12-04T21:30:00+08:00"
  if _,err:=store.ReconcileGig(g,storeTime().Add(time.Hour));err!=nil{t.Fatal(err)}
  if _,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"start","",storeTime());err==nil{
    t.Fatal("stale revision allowed")
  }
  if _,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",2,"start","",storeTime());err!=nil{t.Fatal(err)}
}
func TestPrivateJobTransitionPersistsAcrossRevisionsAndCancellation(t *testing.T){
  store:=NewCampaignStore();g:=sampleGig()
  if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  if _,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"start","",storeTime());err!=nil{t.Fatal(err)}
  if _,err:=store.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,"complete",privateDigest,storeTime());err!=nil{t.Fatal(err)}
  g.BookingStatus="cancelled"
  c,err:=store.ReconcileGig(g,storeTime().Add(time.Hour));if err!=nil{t.Fatal(err)}
  if len(c.Campaign.JobHistory)!=2{t.Fatal("lost history during cancellation")}
  if _,err:=store.TransitionPrivateJob(g.GigID,"withdraw-approved-content",2,"complete",privateDigest,storeTime());err==nil{
     t.Fatal("external withdrawal accepted without dedicated authorized executor")
  }
  if len(store.Campaigns[g.GigID].Jobs)!=2 {t.Fatal("cancelled gig still has old promotion jobs")}
}
func TestPrivateJobTransitionFileRoundTrip(t *testing.T){
  path:=privateStorePath(t)
  g:=sampleGig()
  if _,err:=ApplyGigIntakeFile(path,g,storeTime());err!=nil{t.Fatal(err)}
  a,err:=ApplyPrivateJobTransitionFile(path,g.GigID,"reconcile-gig-identity",1,"start","",storeTime())
  if err!=nil{t.Fatal(err)}
  if !a.LocalStateWritten||a.PublicWrites!=0{t.Fatal("private transition not persisted or indicated public write")}
  b,err:=ApplyPrivateJobTransitionFile(path,g.GigID,"reconcile-gig-identity",1,"complete",privateDigest,storeTime())
  if err!=nil{t.Fatal(err)}
  if b.Change!="private-complete"{t.Fatal("private job did not finish")}
  c,err:=LoadCampaignStore(path);if err!=nil{t.Fatal(err)}
  if len(c.Campaigns[g.GigID].JobHistory)!=2||c.Campaigns[g.GigID].Jobs["reconcile-gig-identity"].State!="private-complete"{
     t.Fatalf("stored job state not recovered: %+v",c.Campaigns[g.GigID].JobHistory)
  }
}
func TestPrivateJobTransitionDisallowsInvalidOperations(t *testing.T){
  s:=NewCampaignStore();g:=sampleGig()
  if _,err:=s.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  for _,action:=range []string{"approve","publish","delete","complete"}{
    _,err:=s.TransitionPrivateJob(g.GigID,"reconcile-gig-identity",1,action,privateDigest,storeTime())
    if err==nil{t.Fatalf("accepted unauthorized action %s",action)}
    if action=="complete"&&!strings.Contains(err.Error(),"not started"){t.Fatal("unexpected error for completion before start")}
  }
  if len(s.Campaigns[g.GigID].JobHistory)!=0 {t.Fatal("invalid attempts mutated history")}
}
