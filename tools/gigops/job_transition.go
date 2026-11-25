package gigops

// Private-only job state transitions. This is not an executor. No action can
// advance an external publisher, user-contact, paid action or physical task.
import (
  "errors"
  "fmt"
  "os"
  "path/filepath"
  "syscall"
  "time"
)

type PrivateJobTransitionEvent struct {
  JobID string `json:"job_id"`
  Revision uint64 `json:"revision"`
  State string `json:"state"`
  At string `json:"at"`
  ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
}
type PrivateJobTransitionResult struct {
  GigID string `json:"gig_id"`
  Job CampaignJobRecord `json:"job"`
  Change string `json:"change"`
  LocalStateWritten bool `json:"local_state_written"`
  PublicWrites int `json:"public_writes"`
}

func validatePrivateJobHistory(c StoredCampaign)error {
  if len(c.JobHistory)>200{return errors.New("job audit exceeds 200 records")}
  for _,e:=range c.JobHistory {
    if e.JobID==""||e.Revision==0||e.Revision>c.Revision||
      !allowed(e.State,"in-progress","private-complete"){
      return errors.New("invalid job audit entry")
    }
    if _,err:=time.Parse(time.RFC3339,e.At);err!=nil{return errors.New("invalid job event time")}
    if e.State=="private-complete"&&!storedFingerprintRE.MatchString(e.ArtifactSHA256){
      return errors.New("private-complete job missing SHA256")
    }
  }
  for id,job:=range c.Jobs {
    if id!=job.ID||job.SourceRevision!=c.Revision {return errors.New("invalid current job identity/revision")}
    if job.State=="private-complete"&&!storedFingerprintRE.MatchString(job.ArtifactSHA256){
       return errors.New("job claims internal completion without a SHA256")
    }
  }
  return nil
}
func (s *CampaignStore)TransitionPrivateJob(gigID,jobID string,expectedRevision uint64,action,artifactSHA256 string,at time.Time)(PrivateJobTransitionResult,error) {
  if s.SchemaVersion!=campaignStoreVersion{return PrivateJobTransitionResult{},errors.New("invalid store version")}
  c,ok:=s.Campaigns[gigID]
  if !ok{return PrivateJobTransitionResult{},errors.New("unknown gig")}
  if expectedRevision==0||expectedRevision!=c.Revision{
    return PrivateJobTransitionResult{},errors.New("stale campaign revision; re-read before updating")
  }
  job,ok:=c.Jobs[jobID]
  if !ok{return PrivateJobTransitionResult{},errors.New("unknown job")}
  if job.Execution!="automatic-draft"{
    return PrivateJobTransitionResult{},errors.New("only private draft work can advance locally; external/physical jobs need a dedicated authorized executor")
  }
  if c.Gig.BookingStatus!="confirmed" && job.ID!="reconcile-gig-identity"{
    return PrivateJobTransitionResult{},errors.New("non-confirmed gig may not advance downstream private jobs")
  }
  if action=="complete"&&job.State=="private-complete"&&artifactSHA256==job.ArtifactSHA256{
    return PrivateJobTransitionResult{GigID:gigID,Job:job,Change:"unchanged",PublicWrites:0},nil
  }
  newState:=""
  switch action {
  case "start":
    if job.State!="ready"{return PrivateJobTransitionResult{},fmt.Errorf("job %s is not ready (state %s)",jobID,job.State)}
    for _,dep:=range job.DependsOn{
      upstream,ok:=c.Jobs[dep]
      if !ok||upstream.State!="private-complete"{
        return PrivateJobTransitionResult{},fmt.Errorf("dependency %s is not privately complete",dep)
      }
    }
    if job.Attempts>=100{return PrivateJobTransitionResult{},errors.New("private job attempt limit reached")}
    job.Attempts++
    job.StartedAt=at.UTC().Format(time.RFC3339)
    newState="in-progress"
  case "complete":
    if job.State!="in-progress"{return PrivateJobTransitionResult{},fmt.Errorf("job %s has not started",jobID)}
    if !storedFingerprintRE.MatchString(artifactSHA256){
      return PrivateJobTransitionResult{},errors.New("internal completion requires artifact SHA256 (64 lowercase hex digits)")
    }
    job.ArtifactSHA256=artifactSHA256
    job.FinishedAt=at.UTC().Format(time.RFC3339)
    newState="private-complete"
  default:
    return PrivateJobTransitionResult{},errors.New("only start or complete private job actions are supported")
  }
  job.State=newState
  c.Jobs[jobID]=job
  c.JobHistory=append(c.JobHistory,PrivateJobTransitionEvent{
    JobID:jobID,Revision:c.Revision,State:newState,At:at.UTC().Format(time.RFC3339),
    ArtifactSHA256:job.ArtifactSHA256,
  })
  if len(c.JobHistory)>200{c.JobHistory=c.JobHistory[len(c.JobHistory)-200:]}
  c.UpdatedAt=at.UTC().Format(time.RFC3339)
  if err:=validatePrivateJobHistory(c);err!=nil{return PrivateJobTransitionResult{},err}
  s.Campaigns[gigID]=c
  return PrivateJobTransitionResult{GigID:gigID,Job:job,Change:newState,PublicWrites:0},nil
}
func ApplyPrivateJobTransitionFile(path,gigID,jobID string,revision uint64,action,artifactSHA256 string,now time.Time)(PrivateJobTransitionResult,error){
  if err:=checkCampaignFilePath(path);err!=nil{return PrivateJobTransitionResult{},err}
  if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return PrivateJobTransitionResult{},err}
  lockPath:=path+".lock"
  if st,err:=os.Lstat(lockPath);err==nil&&(st.Mode()&os.ModeSymlink)!=0{
    return PrivateJobTransitionResult{},errors.New("refusing symlink lock path")
  }else if err!=nil&&!errors.Is(err,os.ErrNotExist){return PrivateJobTransitionResult{},err}
  file,err:=os.OpenFile(lockPath,os.O_CREATE|os.O_RDWR,0600)
  if err!=nil{return PrivateJobTransitionResult{},err}
  defer file.Close()
  if err:=syscall.Flock(int(file.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);err!=nil{
    return PrivateJobTransitionResult{},fmt.Errorf("campaign store already locked: %w",err)
  }
  defer syscall.Flock(int(file.Fd()),syscall.LOCK_UN)
  store,err:=LoadCampaignStore(path);if err!=nil{return PrivateJobTransitionResult{},err}
  result,err:=store.TransitionPrivateJob(gigID,jobID,revision,action,artifactSHA256,now)
  if err!=nil{return PrivateJobTransitionResult{},err}
  if result.Change!="unchanged" {
    if err:=SaveCampaignStore(path,store);err!=nil{return PrivateJobTransitionResult{},err}
    result.LocalStateWritten=true
  }
  return result,nil
}
