package gigops

// CampaignStore is private, local, and deliberately incapable of publishing.
// It persists one campaign and its internal jobs for each durable gig identity.
// External publication evidence stays in the verified listing ledger until a
// trusted readback adapter exists.
import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "syscall"
    "time"
)

const campaignStoreVersion = 1
var durableGigID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,79}$`)
var storedFingerprintRE = regexp.MustCompile(`^[a-f0-9]{64}$`)

type CampaignJobRecord struct {
    ID string `json:"id"`
    Execution string `json:"execution"`
    State string `json:"state"`
    SourceRevision uint64 `json:"source_revision"`
    DependsOn []string `json:"depends_on,omitempty"`
    Next string `json:"next"`
    Attempts uint32 `json:"attempts,omitempty"`
    ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
    StartedAt string `json:"started_at,omitempty"`
    FinishedAt string `json:"finished_at,omitempty"`
}
type CampaignRevision struct {
    Number uint64 `json:"number"`
    Fingerprint string `json:"fingerprint"`
    Status string `json:"status"`
    SupersededAt string `json:"superseded_at"`
}
type StoredCampaign struct {
    Gig GigIntake `json:"gig"`
    Fingerprint string `json:"fingerprint"`
    Revision uint64 `json:"revision"`
    UpdatedAt string `json:"updated_at"`
    Jobs map[string]CampaignJobRecord `json:"jobs"`
    MetricoolObservations []MetricoolStoredObservation `json:"metricool_observations,omitempty"`
    JobHistory []PrivateJobTransitionEvent `json:"job_history,omitempty"`
    History []CampaignRevision `json:"history,omitempty"`
}
type CampaignStore struct {
    SchemaVersion int `json:"schema_version"`
    Campaigns map[string]StoredCampaign `json:"campaigns"`
}
type CampaignWriteResult struct {
    Change string `json:"change"` // created, unchanged, revised, cancelled
    LocalStateWritten bool `json:"local_state_written"` // does not imply external writes
    PublicWrites int `json:"public_writes"`
    Campaign StoredCampaign `json:"campaign"`
    Plan GigIntakePlan `json:"plan"`
}

func NewCampaignStore() CampaignStore {
    return CampaignStore{SchemaVersion:campaignStoreVersion, Campaigns:make(map[string]StoredCampaign)}
}
func DefaultCampaignStorePath()(string,error){
    base,err:=os.UserConfigDir();if err!=nil{return "",err}
    return filepath.Join(base,"balkazar-musicops","campaigns.json"),nil
}
func gigFingerprint(g GigIntake)string{
    b,_:=json.Marshal(g) // fixed validated public-safe struct
    digest:=sha256.Sum256(b)
    return hex.EncodeToString(digest[:])
}
func jobRecords(plan GigIntakePlan,revision uint64)map[string]CampaignJobRecord{
    jobs:=make(map[string]CampaignJobRecord,len(plan.Steps))
    for _,s:=range plan.Steps{
        state:=s.State
        if state=="needs-authorization"{state="approval-required"}
        jobs[s.ID]=CampaignJobRecord{ID:s.ID,Execution:s.Execution,State:state,
            SourceRevision:revision,DependsOn:append([]string(nil),s.DependsOn...),Next:s.Next}
    }
    return jobs
}

// ReconcileGig saves a private proposal. Missing facts remain blockers.
// Repeated intake with the same durable ID is idempotent. Changed facts
// trigger review of existing events and posts instead of duplicate creates.
func(s *CampaignStore)ReconcileGig(g GigIntake,now time.Time)(CampaignWriteResult,error){
    if !durableGigID.MatchString(g.GigID){
        return CampaignWriteResult{},errors.New("stable gig_id required: lowercase slug (3-80 chars); reconcile existing booking first")
    }
    p,err:=PlanGigIntake(g,now);if err!=nil{return CampaignWriteResult{},err}
    if s.SchemaVersion!=campaignStoreVersion{return CampaignWriteResult{},errors.New("unsupported campaign store version")}
    if s.Campaigns==nil{s.Campaigns=make(map[string]StoredCampaign)}
    // Reject exact known source or duplicate venue/time under a different ID.
    for id,record:=range s.Campaigns{
        if id==g.GigID{continue}
        old:=record.Gig
        if g.OfficialSourceURL!=""&&old.OfficialSourceURL==g.OfficialSourceURL{
            return CampaignWriteResult{},fmt.Errorf("canonical event URL already belongs to gig_id %q",id)
        }
        if g.Start!=""&&g.Venue!=""&&old.Artist==g.Artist&&old.Start==g.Start&&
            strings.EqualFold(strings.TrimSpace(old.Venue),strings.TrimSpace(g.Venue)){
            return CampaignWriteResult{},fmt.Errorf("possible duplicate artist/venue/start: existing gig_id %q",id)
        }
    }
    hash:=gigFingerprint(g)
    previous,exists:=s.Campaigns[g.GigID]
    if !exists&&len(s.Campaigns)>=1000{return CampaignWriteResult{},errors.New("campaign store limit reached")}
    if exists&&previous.Fingerprint==hash{
        return CampaignWriteResult{Change:"unchanged",PublicWrites:0,Campaign:previous,Plan:p},nil
    }
    revision:=uint64(1)
    var history []CampaignRevision
    if exists{
        revision=previous.Revision+1
        history=append(append([]CampaignRevision(nil),previous.History...),CampaignRevision{
            Number:previous.Revision,Fingerprint:previous.Fingerprint,Status:previous.Gig.BookingStatus,
            SupersededAt:now.UTC().Format(time.RFC3339),
        })
        if len(history)>40{history=history[len(history)-40:]}
    }
    var observations []MetricoolStoredObservation
    var jobHistory []PrivateJobTransitionEvent
    if exists {
        observations=append(observations,previous.MetricoolObservations...)
        jobHistory=append(jobHistory,previous.JobHistory...)
    }
    jobs:=jobRecords(p,revision)
    if exists&&previous.Gig.BookingStatus=="confirmed"&&g.BookingStatus=="confirmed"{
        for _,name:=range []string{"publish-event-listings","schedule-social-promotion"}{
            if job,ok:=jobs[name];ok{
                job.State="reconciliation-required"
                job.Next="Inspect live and scheduled platform receipts; update the existing event/post only after rights and scope review, never create a duplicate"
                jobs[name]=job
            }
        }
    }
    record:=StoredCampaign{Gig:g,Fingerprint:hash,Revision:revision,
        UpdatedAt:now.UTC().Format(time.RFC3339),Jobs:jobs,MetricoolObservations:observations,JobHistory:jobHistory,History:history}
    s.Campaigns[g.GigID]=record
    change:="created"
    if exists{change="revised"}
    if g.BookingStatus=="cancelled"{change="cancelled"}
    return CampaignWriteResult{Change:change,PublicWrites:0,Campaign:record,Plan:p},nil
}
func checkCampaignFilePath(path string)error{
    if !filepath.IsAbs(path){return errors.New("campaign store path must be absolute")}
    if filepath.Ext(path)!=".json"{return errors.New("campaign store path must be a .json file")}
    for d:=filepath.Dir(path);;d=filepath.Dir(d){
        if _,err:=os.Stat(filepath.Join(d,".git"));err==nil{
            return errors.New("refusing to store private campaign state inside a git repository")
        }
        if d==filepath.Dir(d){break}
    }
    if st,err:=os.Lstat(path);err==nil&&(st.Mode()&os.ModeSymlink)!=0{
        return errors.New("refusing symlink for campaign state")
    }else if err!=nil&&!errors.Is(err,os.ErrNotExist){return err}
    return nil
}
func LoadCampaignStore(path string)(CampaignStore,error){
    if err:=checkCampaignFilePath(path);err!=nil{return CampaignStore{},err}
    f,err:=os.Open(path)
    if errors.Is(err,os.ErrNotExist){return NewCampaignStore(),nil}
    if err!=nil{return CampaignStore{},err}
    defer f.Close()
    info,err:=f.Stat();if err!=nil{return CampaignStore{},err}
    if info.Size()>8<<20{return CampaignStore{},errors.New("campaign store exceeds 8 MiB limit")}
    if info.Mode().Perm()&0077!=0{return CampaignStore{},errors.New("campaign store permissions too broad; expected private 0600 file")}
    var s CampaignStore
    dec:=json.NewDecoder(io.LimitReader(f,8<<20));dec.DisallowUnknownFields()
    if err:=dec.Decode(&s);err!=nil{return CampaignStore{},fmt.Errorf("campaign store invalid: %w",err)}
    var trailing any
    if err:=dec.Decode(&trailing);err!=io.EOF{
        return CampaignStore{},errors.New("campaign store contains trailing JSON or exceeds size limit")
    }
    if s.SchemaVersion!=campaignStoreVersion{return CampaignStore{},errors.New("unsupported campaign store version")}
    if len(s.Campaigns)>1000{return CampaignStore{},errors.New("campaign store too large")}
    if s.Campaigns==nil{s.Campaigns=make(map[string]StoredCampaign)}
    for id,record:=range s.Campaigns{
        if err:=validatePrivateJobHistory(record);err!=nil{
            return CampaignStore{},fmt.Errorf("invalid job history for %q: %w",id,err)
        }
        if err:=validateStoredMetricoolObservations(record.MetricoolObservations);err!=nil{
            return CampaignStore{},fmt.Errorf("invalid Metricool observations for %q: %w",id,err)
        }
        if id!=record.Gig.GigID||!durableGigID.MatchString(id)||record.Revision==0||
            !storedFingerprintRE.MatchString(record.Fingerprint)||record.Fingerprint!=gigFingerprint(record.Gig){
            return CampaignStore{},fmt.Errorf("campaign store contains inconsistent gig %q",id)
        }
    }
    return s,nil
}
func SaveCampaignStore(path string,s CampaignStore)error{
    if err:=checkCampaignFilePath(path);err!=nil{return err}
    if s.SchemaVersion!=campaignStoreVersion{return errors.New("unsupported campaign store version")}
    dir:=filepath.Dir(path)
    if err:=os.MkdirAll(dir,0700);err!=nil{return err}
    tmp,err:=os.CreateTemp(dir,".campaign-*.tmp");if err!=nil{return err}
    defer os.Remove(tmp.Name())
    if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
    enc:=json.NewEncoder(tmp);enc.SetIndent("","  ")
    if err:=enc.Encode(s);err!=nil{tmp.Close();return err}
    if err:=tmp.Sync();err!=nil{tmp.Close();return err}
    if err:=tmp.Close();err!=nil{return err}
    if err:=os.Rename(tmp.Name(),path);err!=nil{return err}
    return nil
}

// ApplyGigIntakeFile holds an exclusive local lock, reconciles and atomically
// saves. No network, third-party publication, API key or other account write.
func ApplyGigIntakeFile(path string,g GigIntake,now time.Time)(CampaignWriteResult,error){
    if err:=checkCampaignFilePath(path);err!=nil{return CampaignWriteResult{},err}
    if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return CampaignWriteResult{},err}
    lockPath:=path+".lock"
    if st,err:=os.Lstat(lockPath);err==nil&&(st.Mode()&os.ModeSymlink)!=0{
        return CampaignWriteResult{},errors.New("refusing symlink lock path")
    }
    lockFile,err:=os.OpenFile(lockPath,os.O_CREATE|os.O_RDWR,0600)
    if err!=nil{return CampaignWriteResult{},err}
    defer lockFile.Close()
    if err:=syscall.Flock(int(lockFile.Fd()),syscall.LOCK_EX|syscall.LOCK_NB);err!=nil{
        return CampaignWriteResult{},fmt.Errorf("campaign store is already locked: %w",err)
    }
    defer syscall.Flock(int(lockFile.Fd()),syscall.LOCK_UN)
    store,err:=LoadCampaignStore(path);if err!=nil{return CampaignWriteResult{},err}
    r,err:=store.ReconcileGig(g,now);if err!=nil{return CampaignWriteResult{},err}
    if r.Change!="unchanged"{
        if err:=SaveCampaignStore(path,store);err!=nil{return CampaignWriteResult{},err}
        r.LocalStateWritten=true
    }
    return r,nil
}
