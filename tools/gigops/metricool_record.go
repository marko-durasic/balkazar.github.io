package gigops

// Persist metadata derived from a caller-supplied Metricool planner readback.
// This is a private observation log, not publication verification, a publisher,
// or an approval grant. Raw text/media and account credentials are not stored.
import (
  "errors"
  "fmt"
  "io"
  "os"
  "path/filepath"
  "reflect"
  "syscall"
  "time"
)

type MetricoolObservationVersion struct {
  PlannerPostID string `json:"planner_post_id"`
  State string `json:"state"`
  ContentSHA256 string `json:"content_sha256"`
  ObservedAt string `json:"observed_at"`
}
type MetricoolStoredObservation struct {
  PostUUID string `json:"post_uuid"`
  PlannerPostID string `json:"planner_post_id"`
  State string `json:"state"`
  AutoPublish bool `json:"auto_publish"`
  PublicationAt string `json:"publication_at"`
  Networks []string `json:"networks"`
  ContentSHA256 string `json:"content_sha256"`
  ObservedAt string `json:"observed_at"`
  SourceRevision uint64 `json:"source_revision"`
  Previous []MetricoolObservationVersion `json:"previous,omitempty"`
}
type MetricoolRecordResult struct {
  GigID string `json:"gig_id"`
  ObservationChanges int `json:"observation_changes"`
  ObservationsTotal int `json:"observations_total"`
  LocalStateWritten bool `json:"local_state_written"`
  PublicWrites int `json:"public_writes"`
  VerifiedPublications int `json:"verified_publications"`
  Preview MetricoolReadbackPreview `json:"preview"`
}

func validateStoredMetricoolObservations(observations []MetricoolStoredObservation)error {
  if len(observations)>300{return errors.New("too many Metricool observations")}
  ids:=map[string]bool{}
  for _,o:=range observations {
    if !metricoolUUIDRE.MatchString(o.PostUUID) || !metricoolPostIDRE.MatchString(o.PlannerPostID) ||
      !allowed(o.State,"observed-draft","observed-queued","needs-review") ||
      !storedFingerprintRE.MatchString(o.ContentSHA256) || o.SourceRevision==0 {
      return errors.New("invalid Metricool observation identity/state")
    }
    if ids[o.PostUUID]{return errors.New("duplicate Metricool post UUID in local campaign")}
    ids[o.PostUUID]=true
    if _,err:=time.Parse(time.RFC3339,o.ObservedAt);err!=nil{return errors.New("invalid observation timestamp")}
    if _,err:=time.Parse(time.RFC3339,o.PublicationAt);err!=nil{return errors.New("invalid publication schedule timestamp")}
    if len(o.Networks)==0||len(o.Networks)>10{return errors.New("invalid observation network count")}
    for _,network:=range o.Networks{
      if !allowed(network,"facebook","instagram","threads","youtube","tiktok","bluesky","twitter","linkedin","pinterest","gmb"){
        return errors.New("invalid observed network")
      }
    }
    if len(o.Previous)>8{return errors.New("observation history exceeds bound")}
  }
  return nil
}
func sameMetricoolObservation(o MetricoolStoredObservation,p MetricoolPostObservation)bool {
  return o.PlannerPostID==p.PlannerPostID &&
    o.State==p.State && o.AutoPublish==p.AutoPublish &&
    o.PublicationAt==p.PublicationAt && o.ContentSHA256==p.ContentSHA256 &&
    reflect.DeepEqual(o.Networks,p.Networks)
}

// RecordMetricoolReadback checks an entire preview before modifying the store.
// UUIDs cannot silently migrate between different gigs. Observing a queued post
// does not alter its approval-required publishing job to "completed".
func(s *CampaignStore)RecordMetricoolReadback(preview MetricoolReadbackPreview,now time.Time)(MetricoolRecordResult,error){
  if s.SchemaVersion!=campaignStoreVersion{return MetricoolRecordResult{},errors.New("unsupported store version")}
  g,ok:=s.Campaigns[preview.GigID]
  if !ok{return MetricoolRecordResult{},errors.New("gig not in private campaign store")}
  if !preview.ReadOnly||preview.PublicWrites!=0||preview.Source!="supplied-connector-snapshot"||
     preview.BrandID!=balkazarMetricoolBrandID||preview.EventURL!=g.Gig.OfficialSourceURL {
     return MetricoolRecordResult{},errors.New("readback is not scoped to this gig and Balkazar brand")
  }
  for id,other:=range s.Campaigns {
    if id==preview.GigID{continue}
    for _,previous:=range other.MetricoolObservations{
      for _,post:=range preview.MatchedPosts{
        if previous.PostUUID==post.PostUUID {
          return MetricoolRecordResult{},fmt.Errorf("Metricool UUID already recorded for different gig %q",id)
        }
      }
    }
  }
  existing:=append([]MetricoolStoredObservation(nil),g.MetricoolObservations...)
  if err:=validateStoredMetricoolObservations(existing);err!=nil{return MetricoolRecordResult{},err}
  byUUID:=map[string]int{}
  for i,o:=range existing{byUUID[o.PostUUID]=i}
  changes:=0
  for _,p:=range preview.MatchedPosts{
    next:=MetricoolStoredObservation{
      PostUUID:p.PostUUID,PlannerPostID:p.PlannerPostID,State:p.State,
      AutoPublish:p.AutoPublish,PublicationAt:p.PublicationAt,
      Networks:append([]string(nil),p.Networks...),ContentSHA256:p.ContentSHA256,
      ObservedAt:now.UTC().Format(time.RFC3339),SourceRevision:g.Revision,
    }
    if idx,found:=byUUID[p.PostUUID];found{
      old:=existing[idx]
      if sameMetricoolObservation(old,p){continue}
      next.Previous=append(append([]MetricoolObservationVersion(nil),old.Previous...),MetricoolObservationVersion{
        PlannerPostID:old.PlannerPostID,State:old.State,
        ContentSHA256:old.ContentSHA256,ObservedAt:old.ObservedAt,
      })
      if len(next.Previous)>8{next.Previous=next.Previous[len(next.Previous)-8:]}
      existing[idx]=next
    }else{
      if len(existing)>=300{return MetricoolRecordResult{},errors.New("Metricool observation limit reached")}
      byUUID[p.PostUUID]=len(existing)
      existing=append(existing,next)
    }
    changes++
  }
  if err:=validateStoredMetricoolObservations(existing);err!=nil{return MetricoolRecordResult{},err}
  if changes>0{
    g.MetricoolObservations=existing
    if job,ok:=g.Jobs["schedule-social-promotion"];ok &&
       job.State=="approval-required" {
      job.Next="Metricool planner snapshot contains existing drafts or queued posts; compare by stable UUID before requesting another scheduling approval"
      g.Jobs[job.ID]=job
    }
    g.UpdatedAt=now.UTC().Format(time.RFC3339)
    s.Campaigns[preview.GigID]=g
  }
  return MetricoolRecordResult{
    GigID:preview.GigID,ObservationChanges:changes,
    ObservationsTotal:len(existing),PublicWrites:0,VerifiedPublications:0,Preview:preview,
  },nil
}

// ApplyMetricoolReadbackFile shares the same .lock path as gig intake and
// atomically persists only non-sensitive metadata derived from the snapshot.
// Inputs come from an authenticated connector caller; the file itself is not
// cryptographically attested and must not be used as proof of publication.
func ApplyMetricoolReadbackFile(path,gigID,brandID string,snapshot io.Reader,now time.Time)(MetricoolRecordResult,error){
  if err:=checkCampaignFilePath(path);err!=nil{return MetricoolRecordResult{},err}
  if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return MetricoolRecordResult{},err}
  lockPath:=path+".lock"
  if st,err:=os.Lstat(lockPath);err==nil&&(st.Mode()&os.ModeSymlink)!=0{
    return MetricoolRecordResult{},errors.New("refusing symlink lock path")
  }else if err!=nil&&!errors.Is(err,os.ErrNotExist){return MetricoolRecordResult{},err}
  lock,err:=os.OpenFile(lockPath,os.O_CREATE|os.O_RDWR,0600)
  if err!=nil{return MetricoolRecordResult{},err}
  defer lock.Close()
  if err:=syscall.Flock(int(lock.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);err!=nil{
    return MetricoolRecordResult{},fmt.Errorf("campaign store already locked: %w",err)
  }
  defer syscall.Flock(int(lock.Fd()),syscall.LOCK_UN)
  store,err:=LoadCampaignStore(path);if err!=nil{return MetricoolRecordResult{},err}
  record,ok:=store.Campaigns[gigID]
  if !ok{return MetricoolRecordResult{},errors.New("gig must be in existing campaign store")}
  preview,err:=PreviewMetricoolSnapshot(snapshot,record.Gig,brandID)
  if err!=nil{return MetricoolRecordResult{},err}
  result,err:=store.RecordMetricoolReadback(preview,now)
  if err!=nil{return MetricoolRecordResult{},err}
  if result.ObservationChanges>0{
    if err:=SaveCampaignStore(path,store);err!=nil{return MetricoolRecordResult{},err}
    result.LocalStateWritten=true
  }
  return result,nil
}
