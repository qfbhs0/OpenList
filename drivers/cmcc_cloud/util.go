package cmcc_cloud

import (
"crypto/aes"
"crypto/cipher"
"crypto/md5"
"encoding/base64"
"fmt"
"strconv"
"strings"
"time"

"github.com/OpenListTeam/OpenList/v4/drivers/base"
"github.com/OpenListTeam/OpenList/v4/internal/model"
"github.com/OpenListTeam/OpenList/v4/internal/op"
"github.com/OpenListTeam/OpenList/v4/pkg/utils/random"
log "github.com/sirupsen/logrus"
)

// ==================== 常量 ====================
const (
proxyBaseURL = "https://esfile.doglobal.net"
proxyPath    = "/yun/process"
oauthPageURL = "https://miniapp.yun.139.com"

// API 路径（传给代理的 path 字段）
pathAccessToken1  = "/open-mpplatform/oauth2/accessToken1"
pathRefreshToken  = "/open-mpplatform/oauth2/refreshToken"
pathGetDisk       = "/richlifeApp/devapp/getdisk"
pathDownloadReq   = "/richlifeApp/devapp/downloadRequest"
pathUploadReq     = "/richlifeApp/devapp/pcUploadFileRequest"
pathDelContent    = "/richlifeApp/devapp/delCatalogContent"
pathMoveContent   = "/richlifeApp/devapp/moveContentCatalog"
pathCreateCatalog = "/richlifeApp/devapp/createCatalog"
pathUpdateContent = "/richlifeApp/devapp/updateContentInfo"
pathUpdateCatalog = "/richlifeApp/devapp/updateCatalogInfo"
pathGetUserInfo   = "/richlifeApp/devapp/getUserInfo"
pathGetDiskInfo   = "/richlifeApp/devapp/getDiskInfo"

// ES文件浏览器目录层级路径
rootDirName         = "我的文件夹"
appCollectionDirName = "我的应用收藏"
esBrowserDirName     = "ES文件浏览器"
)

// ==================== HTTP 请求封装（走 ES 代理） ====================
func (d *CmccCloud) postAPI(path string, jsonBody interface{}, paramType string, result interface{}) error {
reqBody := map[string]interface{}{
"path":        path,
"jsonBody":    jsonBody,
"paramType":   paramType,
"accessToken": d.Addition.AccessToken,
"deviceId":    d.Addition.DeviceId,
"channelId":   d.Addition.ChannelId,
}
log.Debugf("[cmcc_cloud] postAPI: path=%s, paramType=%s", path, paramType)
resp, err := base.RestyClient.R().
SetHeader("Content-Type", "application/json; charset=utf-8").
SetBody(reqBody).
SetResult(result).
Post(proxyBaseURL + proxyPath)
if err != nil {
return fmt.Errorf("proxy request failed: %w", err)
}
if resp.StatusCode() >= 400 {
return fmt.Errorf("proxy http %d: %s", resp.StatusCode(), resp.String())
}
log.Debugf("[cmcc_cloud] postAPI response: %s", resp.String()[:min(500, len(resp.String()))])
return nil
}

func min(a, b int) int {
if a < b {
return a
}
return b
}

// ==================== ES浏览器临时目录初始化 ====================
// initTempDir 在Init阶段查找并缓存ES文件浏览器目录ID
// 路径：根目录 -> 我的文件夹 -> 我的应用收藏 -> ES文件浏览器
func (d *CmccCloud) initTempDir() error {
if d.esBrowserDirID != "" {
return nil // 已缓存
}

// Step1: 从根目录找"我的文件夹"
rootFileID, err := d.findDirByName("root", rootDirName)
if err != nil {
return fmt.Errorf("find root dir '%s' failed: %w", rootDirName, err)
}
if rootFileID == "" {
return fmt.Errorf("root dir '%s' not found", rootDirName)
}
log.Infof("[cmcc_cloud] initTempDir: found '%s' ID=%s", rootDirName, rootFileID)

// Step2: 从"我的文件夹"找"我的应用收藏"
appCollID, err := d.findDirByName(rootFileID, appCollectionDirName)
if err != nil {
return fmt.Errorf("find app collection dir failed: %w", err)
}
if appCollID == "" {
return fmt.Errorf("app collection dir '%s' not found", appCollectionDirName)
}
log.Infof("[cmcc_cloud] initTempDir: found '%s' ID=%s", appCollectionDirName, appCollID)

// Step3: 从"我的应用收藏"找"ES文件浏览器"
esDirID, err := d.findDirByName(appCollID, esBrowserDirName)
if err != nil {
return fmt.Errorf("find ES browser dir failed: %w", err)
}
if esDirID == "" {
return fmt.Errorf("ES browser dir '%s' not found", esBrowserDirName)
}

d.esBrowserDirID = esDirID
log.Infof("[cmcc_cloud] initTempDir: ES browser dir ID=%s", esDirID)
return nil
}

// findDirByName 在指定父目录下查找名称为name的子目录，返回其catalogID
func (d *CmccCloud) findDirByName(parentCatalogID, name string) (string, error) {
req := GetDiskReq{
CatalogID:       parentCatalogID,
FilterType:      0,
CatalogSortType: 0,
ContentType:     0,
ContentSortType: 0,
SortDirection:   0,
StartNumber:     1,
EndNumber:       200,
CatalogType:     -1,
}
var result GetDiskResp
if err := d.postAPI(pathGetDisk, req, "xml", &result); err != nil {
return "", fmt.Errorf("list dir failed: %w", err)
}
if isAPIError(result.ResultCode) {
return "", fmt.Errorf("list dir api error: resultCode=%s", result.ResultCode)
}
if result.GetDiskResult == nil || result.GetDiskResult.CatalogList == nil {
return "", nil // 无子目录
}
for _, catalog := range result.GetDiskResult.CatalogList.CatalogInfo {
if catalog.CatalogName == name {
return catalog.CatalogID, nil
}
}
return "", nil // 未找到
}

// ==================== Status 管理 ====================
func (d *CmccCloud) setTempStatus(status string) {
if d.statusTimer != nil {
d.statusTimer.Stop()
}
d.statusTimer = time.AfterFunc(200*time.Millisecond, func() {
d.GetStorage().SetStatus(status)
op.MustSaveDriverStorage(d)
})
}

// ==================== UUID 生成 ====================
func generateUUID(appId string) string {
now := time.Now()
ts := now.Format("20060102150405") + fmt.Sprintf("%03d", now.Nanosecond()/1000000)
rand3 := fmt.Sprintf("%03d", random.Rand.Intn(1000))
raw := appId + "_" + ts + rand3
return base64.StdEncoding.EncodeToString([]byte(raw))
}

func generateDeviceId() string {
return random.String(8) + "-" +
random.String(4) + "-" +
random.String(4) + "-" +
random.String(4) + "-" +
random.String(12)
}

// ==================== OAuth 授权 URL 生成 ====================
func (d *CmccCloud) buildAuthURL(uuid string) string {
return fmt.Sprintf(
"%s/middle/index.html#/middlePage?pageType=3&deviceId=%s&appId=%s&appKey=%s&appTitle=OpenList&version=1.0&uuid=%s&redirectUrl=http://localhost",
oauthPageURL, d.Addition.DeviceId, d.Addition.AppId, d.Addition.AppKey, uuid,
)
}

// ==================== Token 轮询 ====================
func (d *CmccCloud) pollToken(uuid string) {
go func() {
const maxTry = 36
const interval = 5 * time.Second
for i := 0; i < maxTry; i++ {
time.Sleep(interval)
var resp AccessToken1Resp
req := AccessToken1Req{UUID: uuid}
if err := d.postAPI(pathAccessToken1, req, "json", &resp); err != nil {
log.Debugf("[cmcc_cloud] pollToken attempt %d failed: %v", i+1, err)
continue
}
if resp.Data != nil && resp.Data.AccessToken != "" {
d.Addition.AccessToken = resp.Data.AccessToken
d.Addition.OAuthUUID = ""
d.GetStorage().SetStatus("work")
op.MustSaveDriverStorage(d)
log.Infof("[cmcc_cloud] OAuth token obtained successfully")
// token获取成功后，也初始化ES浏览器目录
if err := d.initTempDir(); err != nil {
log.Warnf("[cmcc_cloud] init temp dir after pollToken failed: %v", err)
}
return
}
log.Debugf("[cmcc_cloud] pollToken attempt %d: no token yet", i+1)
}
d.GetStorage().SetStatus("授权超时(3分钟)，请在管理后台重新保存此驱动以重试")
op.MustSaveDriverStorage(d)
log.Warnf("[cmcc_cloud] pollToken timed out after %d attempts", maxTry)
}()
}

// ==================== AES 解密 ====================
func aesDecrypt(ciphertext, secretkey string) (string, error) {
h := md5.Sum([]byte(secretkey))
key := fmt.Sprintf("%x", h)
cipherBytes, err := base64.StdEncoding.DecodeString(ciphertext)
if err != nil {
return "", fmt.Errorf("base64 decode failed: %w", err)
}
block, err := aes.NewCipher([]byte(key))
if err != nil {
return "", fmt.Errorf("create aes cipher failed: %w", err)
}
if len(cipherBytes) < aes.BlockSize {
return "", fmt.Errorf("ciphertext too short")
}
iv := []byte(key[:aes.BlockSize])
mode := cipher.NewCBCDecrypter(block, iv)
mode.CryptBlocks(cipherBytes, cipherBytes)
cipherBytes = pkcs7Unpad(cipherBytes)
return string(cipherBytes), nil
}

func pkcs7Unpad(data []byte) []byte {
if len(data) == 0 {
return data
}
padding := int(data[len(data)-1])
if padding > len(data) || padding == 0 {
return data
}
return data[:len(data)-padding]
}

// ==================== 对象转换辅助 ====================
func catalogToObj(c CatalogInfo) model.Obj {
return &model.Object{
ID:       c.CatalogID,
Name:     c.CatalogName,
IsFolder: true,
Modified: parseTime(c.UpdateTime),
Ctime:    parseTime(c.CreateTime),
}
}

func contentToObj(c ContentInfo) model.Obj {
return &model.Object{
ID:       c.ContentID,
Name:     c.ContentName,
Size:     parseSize(c.ContentSize),
IsFolder: false,
Modified: parseTime(c.UpdateTime),
Ctime:    parseTime(c.CreateTime),
}
}

func parseTime(s string) time.Time {
if s == "" {
return time.Time{}
}
for _, layout := range []string{
"20060102150405",
"2006-01-02 15:04:05",
"2006-01-02T15:04:05",
time.RFC3339,
} {
if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
return t
}
}
if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
return time.Unix(ts, 0)
}
return time.Time{}
}

func (a *Addition) sortParams() (catalogSortType, contentSortType, sortDirection int) {
sortDirection = 0
if a.OrderDirection == "desc" {
sortDirection = 1
}
switch a.OrderBy {
case "updateTime":
catalogSortType = 0
contentSortType = 0
case "name":
catalogSortType = 1
contentSortType = 1
case "type":
catalogSortType = 2
contentSortType = 2
case "size":
catalogSortType = 3
contentSortType = 3
default:
catalogSortType = 0
contentSortType = 0
}
return
}

func isAPIError(resultCode string) bool {
return resultCode != "" && resultCode != "0" && !strings.HasPrefix(resultCode, "0")
}

func parseSize(s string) int64 {
if s == "" {
return 0
}
n, err := strconv.ParseInt(s, 10, 64)
if err == nil {
return n
}
return 0
}
