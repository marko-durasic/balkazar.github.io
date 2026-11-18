package gigops

import (
  "errors"
  "os"
  "path/filepath"
  "strings"
  "syscall"
  "testing"
  "time"
)
func storeTime()time.Time{return time.Date(2026,11,14,1,15,0,0,time.FixedZone("TWT",8*3600))}
func privateStorePath(t *testing.T)string{t.Helper();return filepath.Join(t.TempDir(),"private","campaigns.json")}

func TestCampaignStoreRoundTripNoExternalWrites(t *testing.T){
  path:=privateStorePath(t)
  g:=sampleGig()
  first,err:=ApplyGigIntakeFile(path,g,storeTime());if err!=nil{t.Fatal(err)}
  if first.Change!="created"||!first.LocalStateWritten||first.PublicWrites!=0||first.Campaign.Revision!=1{
    t.Fatalf("unexpected first result: %+v",first)
  }
  if len(first.Campaign.Jobs)!=10 {t.Fatalf("lost jobs: %+v",first.Campaign.Jobs)}
  if first.Campaign.Jobs["publish-event-listings"].State!="approval-required"{t.Fatal("auto-approved third-party publication")}
  st,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
  if st.Mode().Perm()&0077!=0{t.Fatalf("private store permissions too open: %v",st.Mode())}
  old,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
  second,err:=ApplyGigIntakeFile(path,g,storeTime().Add(time.Hour));if err!=nil{t.Fatal(err)}
  if second.Change!="unchanged"||second.LocalStateWritten||second.Campaign.Revision!=1{
    t.Fatalf("repeated gig not idempotent: %+v",second)
  }
  same,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
  if string(old)!=string(same){t.Fatal("idempotent apply rewrote state")}
  recovered,err:=LoadCampaignStore(path);if err!=nil{t.Fatal(err)}
  if len(recovered.Campaigns)!=1||recovered.Campaigns[g.GigID].Gig.Start!=g.Start{
    t.Fatal("recovery lost canonical gig")
  }
}
func TestReschedulePreservesGigIdentityAndRequiresReconciliation(t *testing.T){
  store:=NewCampaignStore()
  g:=sampleGig()
  _,err:=store.ReconcileGig(g,storeTime());if err!=nil{t.Fatal(err)}
  g.Start="2026-12-04T20:00:00+08:00"
  g.End="2026-12-04T21:30:00+08:00"
  changed,err:=store.ReconcileGig(g,storeTime().Add(time.Hour));if err!=nil{t.Fatal(err)}
  if changed.Change!="revised"||len(store.Campaigns)!=1||changed.Campaign.Revision!=2||len(changed.Campaign.History)!=1{
    t.Fatalf("reschedule incorrectly forked gig: %+v",changed)
  }
  for _,id:=range []string{"publish-event-listings","schedule-social-promotion"}{
    j:=changed.Campaign.Jobs[id]
    if j.State!="reconciliation-required"||j.SourceRevision!=2{
      t.Fatalf("reschedule failed to invalidate external work: %+v",j)
    }
  }
}
func TestCancellationNeverQueuesPromotion(t *testing.T){
  store:=NewCampaignStore()
  g:=sampleGig();if _,err:=store.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  g.BookingStatus="cancelled"
  out,err:=store.ReconcileGig(g,storeTime().Add(time.Hour));if err!=nil{t.Fatal(err)}
  if out.Change!="cancelled"||out.PublicWrites!=0||len(out.Campaign.Jobs)!=2{
    t.Fatalf("unsafe cancellation: %+v",out)
  }
  if _,ok:=out.Campaign.Jobs["schedule-social-promotion"];ok{t.Fatal("cancelled show still scheduled promotion")}
  if out.Campaign.Jobs["withdraw-approved-content"].State!="approval-required"{t.Fatal("cancellation attempted unapproved external retract")}
  if out.Campaign.Jobs["reconcile-existing-listings"].State!="needs-verification"{t.Fatal("cancellation skipped checking existing listings")}
}
func TestHeldGigRemainsPrivate(t *testing.T){
  g:=sampleGig();g.BookingStatus="held";g.ConfirmationSource=""
  s:=NewCampaignStore();out,err:=s.ReconcileGig(g,storeTime());if err!=nil{t.Fatal(err)}
  if out.Plan.BookingReady||out.Campaign.Jobs["publish-event-listings"].State!="needs-verification"{
    t.Fatal("held gig public publishing enabled")
  }
}
func TestDuplicateGigDifferentIdentityRefused(t *testing.T){
  s:=NewCampaignStore();g:=sampleGig();if _,err:=s.ReconcileGig(g,storeTime());err!=nil{t.Fatal(err)}
  second:=g;second.GigID="balkazar-second-id"
  _,err:=s.ReconcileGig(second,storeTime())
  if err==nil||!strings.Contains(err.Error(),"canonical event URL"){t.Fatalf("duplicate source accepted: %v",err)}
  second.OfficialSourceURL=""
  _,err=s.ReconcileGig(second,storeTime())
  if err==nil||!strings.Contains(err.Error(),"possible duplicate"){t.Fatalf("duplicate start and venue accepted: %v",err)}
  if len(s.Campaigns)!=1{t.Fatal("mutated store after rejecting duplicate")}
}
func TestMissingOrUnsafeGigIDRejected(t *testing.T){
  s:=NewCampaignStore();g:=sampleGig()
  for _,v:=range []string{"","../escape","../../etc/passwd","UPPER","bad name","ab"}{
    g.GigID=v
    if _,err:=s.ReconcileGig(g,storeTime());err==nil{t.Fatalf("accepted unsafe gig_id %q",v)}
  }
}
func TestCorruptStoreFailClosed(t *testing.T){
  path:=privateStorePath(t);if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{t.Fatal(err)}
  content:=[]byte(`{"schema_version":1,"campaigns":{bad-json`)
  if err:=os.WriteFile(path,content,0600);err!=nil{t.Fatal(err)}
  if _,err:=ApplyGigIntakeFile(path,sampleGig(),storeTime());err==nil{t.Fatal("accepted corrupt store")}
  after,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
  if string(after)!=string(content){t.Fatal("overwrote corrupt state")}
}
func TestConcurrentLockPreventsLostUpdates(t *testing.T){
  path:=privateStorePath(t)
  if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{t.Fatal(err)}
  lock,err:=os.OpenFile(path+".lock",os.O_CREATE|os.O_RDWR,0600);if err!=nil{t.Fatal(err)}
  defer lock.Close()
  if err:=syscall.Flock(int(lock.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);err!=nil{t.Fatal(err)}
  defer syscall.Flock(int(lock.Fd()),syscall.LOCK_UN)
  _,err=ApplyGigIntakeFile(path,sampleGig(),storeTime())
  if err==nil||!strings.Contains(err.Error(),"already locked"){t.Fatalf("concurrent write was not refused: %v",err)}
  if _,err:=os.Stat(path);!errors.Is(err,os.ErrNotExist){t.Fatalf("unexpected campaign data created: %v",err)}
}
func TestRejectSymlinkAndGitPaths(t *testing.T){
  dir:=t.TempDir();target:=filepath.Join(dir,"target.json")
  if err:=os.WriteFile(target,[]byte("{}"),0600);err!=nil{t.Fatal(err)}
  link:=filepath.Join(dir,"link.json")
  if err:=os.Symlink(target,link);err!=nil{t.Fatal(err)}
  if _,err:=ApplyGigIntakeFile(link,sampleGig(),storeTime());err==nil{t.Fatal("accepted symlink")}
  repo:=filepath.Join(dir,"repo")
  if err:=os.MkdirAll(filepath.Join(repo,".git"),0700);err!=nil{t.Fatal(err)}
  if _,err:=ApplyGigIntakeFile(filepath.Join(repo,"secrets","campaigns.json"),sampleGig(),storeTime());err==nil{
    t.Fatal("allowed private campaign store in repo")
  }
}
