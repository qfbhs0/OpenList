package cmcc_cloud

import (
"context"
"errors"
"fmt"
"io"
"net/http"
"strconv"
"strings"
"time"

"github.com/OpenListTeam/OpenList/v4/drivers/base"
"github.com/OpenListTeam/OpenList/v4/internal/driver"
"github.com/OpenListTeam/OpenList/v4/internal/errs"
"github.com/OpenListTeam/OpenList/v4/internal/model"
"github.com/OpenListTeam/OpenList/v4/internal/op"
log "github.com/sirupsen/logrus"
)

// CmccCloud 驱动结构体
type CmccCloud struct {
model.Storage
Addition
statusTimer   *time.Timer
esBrowserDirID string // 缓存ES文件浏览器目录ID，两步法必须
}

func (d *CmccCloud) Config() driver.Config {
return config
}

func (d *CmccCloud) GetAddition() driver.Additional {
return &d.Addition
}

// ==================== Init ====================
func (d *CmccCloud) Init(ctx context.Context) error {
if d.Addition.DeviceId == "" {
d.Addition.DeviceId = generateDeviceId()
op.MustSaveDriverStorage(d)
}

// ---- 已有 AccessToken，尝试刷新 ----
if d.Addition.AccessToken != "" {
if err := d.refreshToken(); err != nil {
log.Warnf("[cmcc_cloud] refresh token failed: %v, need re-auth", err)
d.Addition.AccessToken = ""
d.Addition.OAuthUUID = ""
op.MustSaveDriverStorage(d)
} else {
// token刷新成功，初始化ES浏览器临时目录
if err := d.initTempDir(); err != nil {
log.Warnf("[cmcc_cloud] init temp dir failed: %v", err)
}
return nil
}
}

// ---- 无 AccessToken，但有 OAuthUUID ----
if d.Addition.OAuthUUID != "" {
var resp AccessToken1Resp
req := AccessToken1Req{UUID: d.Addition.OAuthUUID}
if err := d.postAPI(pathAccessToken1, req, "json", &resp); err == nil {
if resp.Data != nil && resp.Data.AccessToken != "" {
d.Addition.AccessToken = resp.Data.AccessToken
d.Addition.OAuthUUID = ""
op.MustSaveDriverStorage(d)
log.Infof("[cmcc_cloud] token obtained via accessToken1 on Init")
if err := d.initTempDir(); err != nil {
log.Warnf("[cmcc_cloud] init temp dir failed: %v", err)
}
return nil
}
}
authURL := d.buildAuthURL(d.Addition.OAuthUUID)
d.setTempStatus(fmt.Sprintf("请访问以下URL授权: %s", authURL))
d.pollToken(d.Addition.OAuthUUID)
return nil
}

// ---- 首次授权 ----
uuid := generateUUID(d.Addition.AppId)
d.Addition.OAuthUUID = uuid
op.MustSaveDriverStorage(d)
authURL := d.buildAuthURL(uuid)
d.setTempStatus(fmt.Sprintf("请访问以下URL授权: %s", authURL))
d.pollToken(uuid)
return nil
}

// refreshToken 刷新 accesstoken
func (d *CmccCloud) refreshToken() error {
log.Debugf("[cmcc_cloud] refreshToken: attempting")
req := RefreshTokenReq{
AppId:        d.Addition.AppId,
AppSecret:    d.Addition.AppSecret,
RefreshToken: d.Addition.AccessToken,
}
var resp RefreshTokenResp
if err := d.postAPI(pathRefreshToken, req, "json", &resp); err != nil {
log.Warnf("[cmcc_cloud] refreshToken: proxy request failed: %v", err)
return err
}
if resp.Data == nil || resp.Data.AccessToken == "" {
log.Warnf("[cmcc_cloud] refreshToken: empty token, code=%d, msg=%s", resp.Code, resp.Message)
return fmt.Errorf("refresh token failed: code=%d, msg=%s", resp.Code, resp.Message)
}
d.Addition.AccessToken = resp.Data.AccessToken
op.MustSaveDriverStorage(d)
log.Infof("[cmcc_cloud] refreshToken: success")
return nil
}

// ==================== Drop ====================
func (d *CmccCloud) Drop(ctx context.Context) error {
if d.statusTimer != nil {
d.statusTimer.Stop()
}
return nil
}

// ==================== List ====================
func (d *CmccCloud) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
catalogID := dir.GetID()
if catalogID == "" {
catalogID = "root"
}
catalogSortType, contentSortType, sortDirection := d.Addition.sortParams()
var files []model.Obj
startNum := 1
pageSize := 200

for {
req := GetDiskReq{
CatalogID:       catalogID,
FilterType:      0,
CatalogSortType: catalogSortType,
ContentType:     0,
ContentSortType: contentSortType,
SortDirection:   sortDirection,
StartNumber:     startNum,
EndNumber:       startNum + pageSize - 1,
CatalogType:     -1,
}
var result GetDiskResp
if err := d.postAPI(pathGetDisk, req, "xml", &result); err != nil {
return nil, fmt.Errorf("list failed: %w", err)
}
if isAPIError(result.ResultCode) {
return nil, fmt.Errorf("list api error: resultCode=%s", result.ResultCode)
}
if result.GetDiskResult == nil {
return nil, errors.New("list: empty getDiskResult")
}
if result.GetDiskResult.CatalogList != nil {
for _, catalog := range result.GetDiskResult.CatalogList.CatalogInfo {
files = append(files, catalogToObj(catalog))
}
}
if result.GetDiskResult.ContentList != nil {
for _, content := range result.GetDiskResult.ContentList.ContentInfo {
files = append(files, contentToObj(content))
}
}
totalNodes, _ := strconv.Atoi(result.GetDiskResult.NodeCount)
if totalNodes <= 0 || startNum+pageSize-1 >= totalNodes {
break
}
startNum += pageSize
}
return files, nil
}

// ==================== Link ====================
func (d *CmccCloud) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
req := DownloadReq{
ContentID:   file.GetID(),
OwnerMSISDN: "",
}
var result DownloadResp
if err := d.postAPI(pathDownloadReq, req, "xml", &result); err != nil {
return nil, fmt.Errorf("download request failed: %w", err)
}
downloadURL := result.String
if downloadURL == "" {
return nil, errors.New("download: empty download URL")
}
if isAPIError(result.ResultCode) {
return nil, fmt.Errorf("download api error: resultCode=%s", result.ResultCode)
}

downloadURL = strings.ReplaceAll(downloadURL, "&amp;", "&")
res, err := base.NoRedirectClient.R().
SetDoNotParseResponse(true).
SetContext(ctx).
Get(downloadURL)
if err != nil {
return &model.Link{URL: downloadURL}, nil
}
defer func() { _ = res.RawBody().Close() }()
if res.StatusCode() == 302 {
location := res.Header().Get("location")
if location != "" {
downloadURL = location
}
}
return &model.Link{URL: downloadURL}, nil
}

// ==================== MakeDir（两步法） ====================
// 代理禁止 createCatalog（resultCode=20001101），必须两步法：
// Step1: pcUploadFileRequest 在ES浏览器目录下创建目录
// Step2: moveContentCatalog 将新目录移到目标位置
func (d *CmccCloud) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
esDirID := d.esBrowserDirID
if esDirID == "" {
return nil, errors.New("mkdir: ES browser dir ID not initialized, please re-save driver")
}

// Step1: 在ES浏览器目录下用 pcUploadFileRequest 创建目录
// 创建目录: totalSize="0", newCatalogName=目录名, parentCatalogID=ES浏览器目录ID
uploadReq := UploadReq{
TotalSize:       "0",
ParentCatalogID: esDirID,
NewCatalogName:  dirName,
UploadContentList: UploadContentWrap{
UploadContentInfo: []UploadContentInfo{
{ContentName: "0", ContentSize: "0"},
},
},
}
var uploadResult UploadResp
if err := d.postAPI(pathUploadReq, uploadReq, "xml", &uploadResult); err != nil {
return nil, fmt.Errorf("mkdir step1 failed: %w", err)
}
if isAPIError(uploadResult.ResultCode) {
return nil, fmt.Errorf("mkdir step1 api error: resultCode=%s", uploadResult.ResultCode)
}
if uploadResult.UploadResult == nil || uploadResult.UploadResult.NewContentIDList == nil {
return nil, errors.New("mkdir step1: empty newContentIDList")
}
if len(uploadResult.UploadResult.NewContentIDList.NewContent) == 0 {
return nil, errors.New("mkdir step1: no new content ID returned")
}
newCatalogID := uploadResult.UploadResult.NewContentIDList.NewContent[0].ContentID
if newCatalogID == "" {
return nil, errors.New("mkdir step1: empty new catalog ID")
}
log.Infof("[cmcc_cloud] mkdir step1: created temp dir %s in ES browser dir", newCatalogID)

// Step2: moveContentCatalog 移到目标目录
moveReq := MoveReq{
NewCatalogID:   parentDir.GetID(),
CatalogInfoList: &IDListWrap{ID: []string{newCatalogID}},
}
var moveResult MoveResp
if err := d.postAPI(pathMoveContent, moveReq, "xml", &moveResult); err != nil {
return nil, fmt.Errorf("mkdir step2 (move) failed: %w", err)
}
if isAPIError(moveResult.ResultCode) {
return nil, fmt.Errorf("mkdir step2 api error: resultCode=%s", moveResult.ResultCode)
}
log.Infof("[cmcc_cloud] mkdir step2: moved dir %s to parent %s", newCatalogID, parentDir.GetID())

return &model.Object{
ID:       newCatalogID,
Name:     dirName,
IsFolder: true,
}, nil
}

// ==================== Move ====================
func (d *CmccCloud) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
req := MoveReq{
NewCatalogID: dstDir.GetID(),
}
if srcObj.IsDir() {
req.CatalogInfoList = &IDListWrap{ID: []string{srcObj.GetID()}}
} else {
req.ContentInfoList = &IDListWrap{ID: []string{srcObj.GetID()}}
}
var result MoveResp
if err := d.postAPI(pathMoveContent, req, "xml", &result); err != nil {
return nil, fmt.Errorf("move failed: %w", err)
}
if isAPIError(result.ResultCode) {
return nil, fmt.Errorf("move api error: resultCode=%s", result.ResultCode)
}
return srcObj, nil
}

// ==================== Rename ====================
func (d *CmccCloud) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
if srcObj.IsDir() {
req := UpdateCatalogInfoReq{
CatalogID:   srcObj.GetID(),
CatalogName: newName,
}
var result UpdateCatalogInfoResp
if err := d.postAPI(pathUpdateCatalog, req, "xml", &result); err != nil {
return nil, fmt.Errorf("rename dir failed: %w", err)
}
if isAPIError(result.ResultCode) {
return nil, fmt.Errorf("rename dir api error: resultCode=%s", result.ResultCode)
}
return &model.Object{
ID:       srcObj.GetID(),
Name:     newName,
IsFolder: true,
Modified: srcObj.ModTime(),
Ctime:    srcObj.CreateTime(),
}, nil
}
req := UpdateContentInfoReq{
ContentID:   srcObj.GetID(),
ContentName: newName,
}
var result UpdateContentInfoResp
if err := d.postAPI(pathUpdateContent, req, "xml", &result); err != nil {
return nil, fmt.Errorf("rename failed: %w", err)
}
if isAPIError(result.ResultCode) {
return nil, fmt.Errorf("rename api error: resultCode=%s", result.ResultCode)
}
return &model.Object{
ID:       srcObj.GetID(),
Name:     newName,
Size:     srcObj.GetSize(),
IsFolder: false,
Modified: srcObj.ModTime(),
Ctime:    srcObj.CreateTime(),
}, nil
}

// ==================== Copy ====================
func (d *CmccCloud) Copy(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
return nil, errs.NotSupport
}

// ==================== Remove ====================
func (d *CmccCloud) Remove(ctx context.Context, obj model.Obj) error {
req := DelReq{
OprReason: "0",
}
if obj.IsDir() {
req.CatalogIDs = &IDListWrap{ID: []string{obj.GetID()}}
} else {
req.ContentIDs = &IDListWrap{ID: []string{obj.GetID()}}
}
var result DelResp
if err := d.postAPI(pathDelContent, req, "xml", &result); err != nil {
return fmt.Errorf("remove failed: %w", err)
}
if isAPIError(result.ResultCode) {
return fmt.Errorf("remove api error: resultCode=%s", result.ResultCode)
}
return nil
}

// ==================== Put（两步法上传） ====================
// 能力目录限制：pcUploadFileRequest 只能在ES文件浏览器目录下调用
// Step1: pcUploadFileRequest 在ES浏览器目录下获取上传URL
// Step1.5: POST 上传文件内容到 redirectionUrl
// Step2: moveContentCatalog 将新文件移到目标目录
func (d *CmccCloud) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
esDirID := d.esBrowserDirID
if esDirID == "" {
return nil, errors.New("upload: ES browser dir ID not initialized, please re-save driver")
}

file, err := stream.CacheFullAndWriter(&up, nil)
if err != nil {
return nil, err
}
fileSize := stream.GetSize()
fileSizeStr := strconv.FormatInt(fileSize, 10)

// Step1: pcUploadFileRequest 在ES浏览器目录下
uploadReq := UploadReq{
TotalSize:       fileSizeStr,
ParentCatalogID: esDirID,
UploadContentList: UploadContentWrap{
UploadContentInfo: []UploadContentInfo{
{
ContentName: stream.GetName(),
ContentSize: fileSizeStr,
},
},
},
}
var uploadResult UploadResp
if err := d.postAPI(pathUploadReq, uploadReq, "xml", &uploadResult); err != nil {
return nil, fmt.Errorf("upload step1 failed: %w", err)
}
if isAPIError(uploadResult.ResultCode) {
return nil, fmt.Errorf("upload step1 api error: resultCode=%s", uploadResult.ResultCode)
}
if uploadResult.UploadResult == nil {
return nil, errors.New("upload step1: empty uploadResult")
}

// 获取新文件ID
var newContentID string
if uploadResult.UploadResult.NewContentIDList != nil && len(uploadResult.UploadResult.NewContentIDList.NewContent) > 0 {
newContentID = uploadResult.UploadResult.NewContentIDList.NewContent[0].ContentID
}
if newContentID == "" {
return nil, errors.New("upload step1: empty new content ID")
}

// Step1.5: 上传文件内容（POST到redirectionUrl）
uploadURL := uploadResult.UploadResult.RedirectionUrl
if uploadURL != "" {
uploadURL = strings.ReplaceAll(uploadURL, "&amp;", "&")
reader := io.LimitReader(file, fileSize)
req2, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, reader)
if err != nil {
return nil, fmt.Errorf("create upload request failed: %w", err)
}
req2.ContentLength = fileSize
req2.Header.Set("Content-Type", "application/octet-stream")
req2.Header.Set("uploadtaskID", uploadResult.UploadResult.UploadTaskID)
req2.Header.Set("contentSize", fileSizeStr)
resp, err := base.HttpClient.Do(req2)
if err != nil {
return nil, fmt.Errorf("upload data failed: %w", err)
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
return nil, fmt.Errorf("upload data failed with status %d", resp.StatusCode)
}
log.Infof("[cmcc_cloud] upload step1.5: uploaded %d bytes to redirectionUrl", fileSize)
} else {
log.Infof("[cmcc_cloud] upload step1.5: no redirectionUrl, isNeedUpload=%s, skipping data upload", uploadResult.UploadResult.IsNeedUpload)
}

// Step2: moveContentCatalog 移到目标目录
moveReq := MoveReq{
NewCatalogID:    dstDir.GetID(),
ContentInfoList: &IDListWrap{ID: []string{newContentID}},
}
var moveResult MoveResp
if err := d.postAPI(pathMoveContent, moveReq, "xml", &moveResult); err != nil {
return nil, fmt.Errorf("upload step2 (move) failed: %w", err)
}
if isAPIError(moveResult.ResultCode) {
return nil, fmt.Errorf("upload step2 api error: resultCode=%s", moveResult.ResultCode)
}
log.Infof("[cmcc_cloud] upload step2: moved file %s to target dir %s", newContentID, dstDir.GetID())

up(100)
return &model.Object{
ID:       newContentID,
Name:     stream.GetName(),
Size:     fileSize,
IsFolder: false,
Modified: stream.ModTime(),
}, nil
}

// ==================== GetDetails ====================
func (d *CmccCloud) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
var result GetDiskInfoResp
if err := d.postAPI(pathGetDiskInfo, struct{}{}, "xml", &result); err != nil {
return nil, fmt.Errorf("get disk info failed: %w", err)
}
if isAPIError(result.ResultCode) {
return nil, fmt.Errorf("get disk info error: resultCode=%s", result.ResultCode)
}
if result.GetDiskInfoResult == nil {
return nil, errors.New("get disk info: empty result")
}
totalMB, _ := strconv.ParseInt(result.GetDiskInfoResult.DiskSize, 10, 64)
freeMB, _ := strconv.ParseInt(result.GetDiskInfoResult.FreeDiskSize, 10, 64)
const mb = 1024 * 1024
return &model.StorageDetails{
DiskUsage: model.DiskUsage{
TotalSpace: totalMB * mb,
UsedSpace:  (totalMB - freeMB) * mb,
},
}, nil
}

// ==================== 接口断言 ====================
var _ driver.Driver = (*CmccCloud)(nil)
var _ driver.MkdirResult = (*CmccCloud)(nil)
var _ driver.MoveResult = (*CmccCloud)(nil)
var _ driver.RenameResult = (*CmccCloud)(nil)
