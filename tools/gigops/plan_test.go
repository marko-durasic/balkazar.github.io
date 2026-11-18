package gigops

import (
  "os"
  "path/filepath"
  "strings"
  "testing"
  "time"
)
const source="https://balkazar.org/events/2026-12-03-european-christmas-market/"
const bands="https://www.bandsintown.com/e/109070616-balkazar-at-xiangti-avenue-plaza"
const songkick="https://www.songkick.com/concerts/1234567-balkazar-taipei"
const fp="eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
func sample() ReviewBundle {return ReviewBundle{SchemaVersion:1,ReadOnly:true,Events:[]ReviewEvent{{URL:source,Fingerprint:fp,Start:"2026-12-03T20:00:00+08:00",End:"2026-12-03T21:30:00+08:00",Venue:"Xiangti Avenue Plaza",SongkickBlockers:[]string{"Artist ID unknown"}}}}}
func TestURLAllowlist(t *testing.T){
  for _,x:=range []string{"http://balkazar.org/events/1/","https://evil.test/events/1/","https://balkazar.org.evil.test/events/1/","https://balkazar.org/events/1/?x=1"}{
    if ValidSourceURL(x){t.Fatalf("invalid source accepted: %s",x)}
  }
  if !ValidSourceURL(source){t.Fatal("valid source rejected")}
}
func TestBundle(t *testing.T){
  path:=filepath.Join(t.TempDir(),"bundle.json")
  data:=`{"schema_version":1,"read_only":true,"events":[{"url":"`+source+`","fingerprint":"`+fp+`","start":"2026-12-03T20:00:00+08:00","end":"2026-12-03T21:30:00+08:00","songkick_blockers":["Artist ID unknown"]}]}`
  if err:=os.WriteFile(path,[]byte(data),0600);err!=nil{t.Fatal(err)}
  b,err:=LoadReviewBundle(path);if err!=nil{t.Fatal(err)}
  p:=Plans(b,NewLedger())
  if len(p)!=1||len(p[0].Platforms)!=2||len(p[0].Platforms[1].Blockers)==0{t.Fatalf("invalid plan: %+v",p)}
}
func TestLiveIdempotenceAndApproval(t *testing.T){
  b:=sample();l:=NewLedger()
  if RecordLive(&l,b,source,"bandsintown",bands,false,time.Now())==nil{t.Fatal("accepted without approval")}
  if err:=RecordLive(&l,b,source,"bandsintown",bands,true,time.Now());err!=nil{t.Fatal(err)}
  if err:=RecordLive(&l,b,source,"bandsintown",bands,true,time.Now());err!=nil{t.Fatal("not idempotent",err)}
  if err:=RecordLive(&l,b,source,"bandsintown","https://www.bandsintown.com/e/99-other",true,time.Now());err==nil{t.Fatal("accepted conflicting live event")}
  if err:=RecordLive(&l,b,source,"songkick",songkick,true,time.Now());err!=nil{t.Fatal(err)}
  for _,p:=range Plans(b,l)[0].Platforms{if p.State!="live"||len(p.Blockers)!=0{t.Fatalf("live event incorrectly blocked: %+v",p)}}
}
func TestRejectUnapprovedEventURLs(t *testing.T){
  for _,x:=range []string{"http://www.songkick.com/concerts/12-show","https://songkick.com.evil.test/concerts/12","https://www.songkick.com/artists/12","https://www.songkick.com/concerts/12?x=1"}{
    if ValidPublicListingURL("songkick",x){t.Errorf("allowed unsafe public URL %s",x)}
  }
  if !ValidPublicListingURL("songkick",songkick)||!ValidPublicListingURL("bandsintown",bands){t.Fatal("valid public URL rejected")}
}
func TestLedgerPrivateAndReReadable(t *testing.T){
  path:=filepath.Join(t.TempDir(),"private","ledger.json")
  l:=NewLedger();if err:=RecordLive(&l,sample(),source,"bandsintown",bands,true,time.Now());err!=nil{t.Fatal(err)}
  if err:=SaveLedger(path,l);err!=nil{t.Fatal(err)}
  info,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
  if info.Mode().Perm()&0077!=0{t.Fatalf("ledger permissions too loose: %v",info.Mode())}
  restored,err:=LoadLedger(path);if err!=nil{t.Fatal(err)}
  if restored.Events[source]["bandsintown"].PublicURL!=bands{t.Fatal("ledger not persisted")}
}
func TestSourceChangedRequiresReview(t *testing.T){
  b:=sample();l:=NewLedger();RecordLive(&l,b,source,"bandsintown",bands,true,time.Now())
  b.Events[0].Fingerprint=strings.Repeat("f",64)
  p:=Plans(b,l)[0].Platforms[0]
  if !strings.Contains(p.NextAction,"review")||len(p.Blockers)==0{t.Fatalf("missing source drift check: %+v",p)}
}
