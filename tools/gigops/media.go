package gigops

// This is a read-only intake checker for snapshots supplied by the authenticated
// Google Drive / Google Sheets connectors. It has no network, upload, delete,
// publication or private credential capability.
import (
    "encoding/csv"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "regexp"
    "sort"
    "strings"
)

type InboxFile struct {
    ID           string `json:"id"`
    Title        string `json:"title"`
    Name         string `json:"name"`
    MIMEType     string `json:"mime_type"`
    FileOrFolder string `json:"file_or_folder"`
}

type catalogEntry struct {
    AssetID string
    Artist  string
    DriveID string
    Name    string
}

type MediaItem struct {
    DriveID string   `json:"drive_id"`
    Name    string   `json:"name"`
    Type    string   `json:"mime_type"`
    State   string   `json:"state"`
    AssetID string   `json:"existing_asset_id,omitempty"`
    Blockers []string `json:"blockers,omitempty"`
}

type MediaReview struct {
    SchemaVersion int `json:"schema_version"`
    ReadOnly bool `json:"read_only"`
    Artist string `json:"artist"`
    New []MediaItem `json:"new"`
    Existing []MediaItem `json:"existing"`
    Skipped []MediaItem `json:"skipped"`
}

var driveIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{12,}$`)

func mediaKind(mime string) bool {
    switch strings.ToLower(strings.TrimSpace(mime)) {
    case "image/jpeg", "image/png", "image/webp", "image/heic", "image/heif",
         "image/avif", "video/mp4", "video/quicktime", "video/webm",
         "video/x-matroska":
        return true
    }
    return false
}

func readInbox(r io.Reader) ([]InboxFile,error) {
    var payload struct {
        Result struct { Files []InboxFile `json:"files"` } `json:"result"`
        Files []InboxFile `json:"files"`
    }
    dec:=json.NewDecoder(io.LimitReader(r, 16<<20))
    if err:=dec.Decode(&payload);err!=nil{return nil,fmt.Errorf("inbox JSON: %w",err)}
    var files []InboxFile
    if payload.Result.Files!=nil{files=payload.Result.Files}else{files=payload.Files}
    if files==nil{return nil,errors.New("inbox JSON must contain a files array (possibly empty)")}
    if len(files)>10000{return nil,errors.New("inbox contains too many files")}
    return files,nil
}

func readCatalog(r io.Reader) ([]catalogEntry,error) {
    reader:=csv.NewReader(io.LimitReader(r, 16<<20))
    reader.FieldsPerRecord=-1
    header,err:=reader.Read()
    if err!=nil{return nil,fmt.Errorf("catalog CSV header: %w",err)}
    fields:=map[string]int{}
    for i,h:=range header {fields[strings.TrimSpace(strings.TrimPrefix(h,"\ufeff"))]=i}
    for _,name:=range []string{"Asset ID","Artist","Drive file ID"}{
        if _,ok:=fields[name];!ok{return nil,fmt.Errorf("catalog missing %q header",name)}
    }
    nameCol,hasName:=fields["Original filename"]
    entries:=[]catalogEntry{}
    ids:=map[string]bool{}
    for {
        row,err:=reader.Read()
        if errors.Is(err,io.EOF){break}
        if err!=nil{return nil,fmt.Errorf("catalog CSV row: %w",err)}
        get:=func(column string)string{ix:=fields[column];if ix>=len(row){return ""};return strings.TrimSpace(row[ix])}
        id:=get("Drive file ID")
        if id=="" {continue}
        if !driveIDRE.MatchString(id){return nil,fmt.Errorf("catalog invalid Drive file ID in row %d",len(entries)+2)}
        if ids[id]{return nil,fmt.Errorf("catalog has duplicate Drive ID %s",id)}
        ids[id]=true
        entry:=catalogEntry{AssetID:get("Asset ID"),Artist:get("Artist"),DriveID:id}
        if hasName&&nameCol<len(row){entry.Name=strings.TrimSpace(row[nameCol])}
        entries=append(entries,entry)
        if len(entries)>10000{return nil,errors.New("catalog contains too many rows")}
    }
    return entries,nil
}

// PlanMedia performs strictly read-only deduplication by provider Drive ID.
// Same filenames are suggestions to compare images/hashes, NEVER proof of
// identical content or a reason to delete any original. No public-ready state
// is possible while rights and participant consent are unverified.
func PlanMedia(inbox io.Reader,catalog io.Reader,artist string)(MediaReview,error){
    if artist!="Balkazar" {return MediaReview{},errors.New("only the Balkazar tenant is configured; other bands require separate permissioned sources")}
    files,err:=readInbox(inbox);if err!=nil{return MediaReview{},err}
    existing,err:=readCatalog(catalog);if err!=nil{return MediaReview{},err}
    byID:=map[string]catalogEntry{}
    byName:=map[string][]catalogEntry{}
    for _,e:=range existing{
        // Cross-tenant file IDs may not be silently treated as Balkazar assets.
        if e.Artist!="Balkazar" {continue}
        byID[e.DriveID]=e
        if e.Name!="" {key:=strings.ToLower(e.Name);byName[key]=append(byName[key],e)}
    }
    result:=MediaReview{SchemaVersion:1,ReadOnly:true,Artist:artist,
        New:[]MediaItem{},Existing:[]MediaItem{},Skipped:[]MediaItem{}}
    seen:=map[string]bool{}
    for _,f:=range files{
        if !driveIDRE.MatchString(f.ID){return MediaReview{},fmt.Errorf("invalid provider Drive ID %q",f.ID)}
        if seen[f.ID]{return MediaReview{},fmt.Errorf("duplicate file ID in inventory: %s",f.ID)}
        seen[f.ID]=true
        name:=f.Title
        if name==""{name=f.Name}
        item:=MediaItem{DriveID:f.ID,Name:name,Type:f.MIMEType}
        if old,ok:=byID[f.ID];ok{
            item.State="already-indexed";item.AssetID=old.AssetID
            result.Existing=append(result.Existing,item)
            continue
        }
        if strings.EqualFold(f.FileOrFolder,"folder")||strings.Contains(f.MIMEType,"google-apps.folder") {
            item.State="folder-ignored";result.Skipped=append(result.Skipped,item);continue
        }
        if name==""||!mediaKind(f.MIMEType){
            item.State="unsupported-or-unnamed";result.Skipped=append(result.Skipped,item);continue
        }
        item.State="new-needs-review"
        item.Blockers=[]string{"Photographer/recordist and audio rights not verified","Participant/minor privacy and usage scope not verified","Event year, precise date and venue not verified"}
        if matches:=byName[strings.ToLower(name)];len(matches)>0 {
            item.State="possible-duplicate-needs-hash"
            item.Blockers=append(item.Blockers,"Name matches a catalog record; confirm content hash or visuals, never delete based on name")
        }
        result.New=append(result.New,item)
    }
    sort.Slice(result.New,func(i,j int)bool{return result.New[i].DriveID<result.New[j].DriveID})
    sort.Slice(result.Existing,func(i,j int)bool{return result.Existing[i].DriveID<result.Existing[j].DriveID})
    sort.Slice(result.Skipped,func(i,j int)bool{return result.Skipped[i].DriveID<result.Skipped[j].DriveID})
    return result,nil
}
