package gigops

import (
    "encoding/json"
    "strings"
    "testing"
)

const goodID="1ZJFr8uIPxVTRnAid4MP4hmvV-snPoEjm"
const secondID="1ONeFCxr7V84Ld2zJ6x8gnaHmKnfqfhox"
const catalogHeader="Asset ID,Artist,Drive file ID,Original filename\n"

func review(t *testing.T,inbox,cat string)(MediaReview,error) {
    t.Helper()
    return PlanMedia(strings.NewReader(inbox),strings.NewReader(cat),"Balkazar")
}

func TestMediaPlannerRejectsDuplicateReupload(t *testing.T){
    jsonInput:=`{"result":{"files":[{"id":"`+goodID+`","title":"existing.mp4","mime_type":"video/mp4"}]}}`
    catalog:=catalogHeader+"BA-M-0005,Balkazar,"+goodID+",9478.mp4\n"
    r,err:=review(t,jsonInput,catalog);if err!=nil{t.Fatal(err)}
    if !r.ReadOnly||r.SchemaVersion!=1||len(r.New)!=0||len(r.Existing)!=1||r.Existing[0].AssetID!="BA-M-0005" {
        t.Fatalf("duplicate incorrectly treated as new: %+v",r)
    }
}
func TestMediaPlannerNewFileNeverPublishable(t *testing.T){
    input:=`{"files":[{"id":"`+secondID+`","title":"crowd.jpg","mime_type":"image/jpeg"}]}`
    r,err:=review(t,input,catalogHeader)
    if err!=nil{t.Fatal(err)}
    if len(r.New)!=1||r.New[0].State!="new-needs-review"||len(r.New[0].Blockers)!=3 {
        t.Fatalf("rights guard missing: %+v",r)
    }
    raw,_:=json.Marshal(r)
    if strings.Contains(string(raw),"ready-to-publish") {t.Fatal("publication was implied")}
}
func TestMediaPlannerFilenameCollisionIsOnlyCandidate(t *testing.T){
    input:=`{"files":[{"id":"`+secondID+`","title":"crowd.jpg","mime_type":"image/jpeg"}]}`
    catalog:=catalogHeader+"BA-M-0001,Balkazar,"+goodID+",crowd.jpg\n"
    r,err:=review(t,input,catalog)
    if err!=nil{t.Fatal(err)}
    if r.New[0].State!="possible-duplicate-needs-hash"||len(r.New[0].Blockers)!=4 {
       t.Fatalf("filename incorrectly treated as verified identical: %+v",r)
    }
}
func TestMediaPlannerRejectsCrossTenant(t *testing.T){
    input:=`{"files":[]}`
    _,err:=PlanMedia(strings.NewReader(input),strings.NewReader(catalogHeader),"Fuzzy Wall")
    if err==nil{t.Fatal("cross-band action accepted")}
}
func TestMediaPlannerCorruptCatalogFailsClosed(t *testing.T){
    input:=`{"files":[{"id":"`+secondID+`","title":"new.jpg","mime_type":"image/jpeg"}]}`
    _,err:=review(t,input,catalogHeader+
       "BA-M-0001,Balkazar,"+goodID+",first.jpg\n"+
       "BA-M-0002,Balkazar,"+goodID+",second.jpg\n")
    if err==nil||!strings.Contains(err.Error(),"duplicate Drive ID"){t.Fatalf("failed to reject non-unique register: %v",err)}
}
func TestMediaPlannerRejectsDuplicateSourceIDs(t *testing.T){
    input:=`{"files":[{"id":"`+secondID+`","title":"A.jpg","mime_type":"image/jpeg"},{"id":"`+secondID+`","title":"B.jpg","mime_type":"image/jpeg"}]}`
    _,err:=review(t,input,catalogHeader)
    if err==nil||!strings.Contains(err.Error(),"duplicate file ID"){t.Fatalf("inventory race missed: %v",err)}
}
func TestMediaPlannerSkipsFoldersAndUnrelatedFiles(t *testing.T){
    input:=`{"files":[{"id":"`+goodID+`","title":"old folder","mime_type":"application/vnd.google-apps.folder","file_or_folder":"folder"},{"id":"`+secondID+`","title":"notes.txt","mime_type":"text/plain"}]}`
    r,err:=review(t,input,catalogHeader)
    if err!=nil{t.Fatal(err)}
    if len(r.New)!=0||len(r.Skipped)!=2{t.Fatalf("invalid files ingested: %+v",r)}
}
func TestMediaPlannerEmptyFilesIsValidNoOp(t *testing.T){
    r,err:=review(t,`{"result":{"files":[]}}`,catalogHeader)
    if err!=nil||len(r.New)!=0||len(r.Existing)!=0||len(r.Skipped)!=0 {
        t.Fatalf("empty inbox must remain no-op: %+v %v",r,err)
    }
}
func TestMediaPlannerMissingSourceArrayIsError(t *testing.T){
    _,err:=review(t,`{"result":{}}`,catalogHeader)
    if err==nil{t.Fatal("silent success on missing Drive list")}
}
