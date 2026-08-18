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
oauthPageURL = "https://miniapp.yun.139.com" // OAuth 授权页面仍直连

// API 路径（传给代理的 path 字段）
pathAccessToken1  = "/open-mpplatform/oauth2/accessToken1"
pathRefreshToken  = "/open-mpplatform/oauth2/refreshToken"
pathGetDisk       = "/richlifeApp/devapp/getdisk"
pathDownloadReq   = "/richlifeApp/devapp/downloadRequest"
pathUploadReq     = "/richlifeApp/devapp/pcUploadFileRequest"
pathDelContent    = "/richlifeApp/devapp/delCatalogContent"
pathMoveContent   = "/richlifeApp/devapp/moveCatalogContent"
pathCreateCatalog = "/richlifeApp/devapp/createCatalog"
pathUpdateContent = "/richlifeApp/devapp/updateContentInfo"
pathUpdateCatalog = "/richlifeApp/devapp/updateCatalogInfo"
pathGetUserInfo   = "/richlifeApp/devapp/getUserInfo"
pathGetDiskInfo   = "/richlifeApp/devapp/getDiskInfo"
)

// ==================== HTTP 请求封装（走 ES 代理） ====================

// postAPI 通过 ES 代理发送 API 请求
// path: API 路径，如 "/richlifeApp/devapp/getdisk"
// jsonBody: 请求体（JSON 对象）
// paramType: "xml"（文件操作）或 "json"（OAuth/Token）
// result: 响应解析目标
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

// min helper
func min(a, b int) int {
if a < b {
return a
}
return b
}

// ==================== Status 管理 ====================

// setTempStatus 延迟覆盖 Status
func (d *CmccCloud) setTempStatus(status string) {
if d.statusTimer != nil {
d.statusTimer.Stop()
}
d.statusTimer = time.AfterFunc(200*time.Millisecond, func() {
d.GetStorage().SetStatus(status)
op.MustSaveDriverStorage(d)
})
}

// ==================== UUID 生成（OAuth 授权用） ====================

// generateUUID 生成 OAuth 授权用的 UUID
// 规则: Base64(appId + "_" + yyyyMMddHHmmssSSS + 3位随机数)
func generateUUID(appId string) string {
now := time.Now()
ts := now.Format("20060102150405") + fmt.Sprintf("%03d", now.Nanosecond()/1000000)
rand3 := fmt.Sprintf("%03d", random.Rand.Intn(1000))
raw := appId + "_" + ts + rand3
return base64.StdEncoding.EncodeToString([]byte(raw))
}

// generateDeviceId 生成设备ID (UUID v4 格式)
func generateDeviceId() string {
return random.String(8) + "-" +
random.String(4) + "-" +
random.String(4) + "-" +
random.String(4) + "-" +
random.String(12)
}

// ==================== OAuth 授权 URL 生成 ====================

// buildAuthURL 构造 OAuth 授权页面 URL（仍直连移动云盘）
func (d *CmccCloud) buildAuthURL(uuid string) string {
return fmt.Sprintf(
"%s/middle/index.html#/middlePage?pageType=3&deviceId=%s&appId=%s&appKey=%s&appTitle=OpenList&version=1.0&uuid=%s&redirectUrl=http://localhost",
oauthPageURL, d.Addition.DeviceId, d.Addition.AppId, d.Addition.AppKey, uuid,
)
}

// ==================== Token 轮询（OAuth 授权后换取 accesstoken） ====================

// pollToken 启动后台 goroutine 轮询 accessToken1 接口（走代理）
func (d *CmccCloud) pollToken(uuid string) {
go func() {
const maxTry = 36                // 最多尝试 36 次
const interval = 5 * time.Second // 每次间隔 5 秒（总计 3 分钟）
for i := 0; i < maxTry; i++ {
time.Sleep(interval)
var resp AccessToken1Resp
req := AccessToken1Req{UUID: uuid}
if err := d.postAPI(pathAccessToken1, req, "json", &resp); err != nil {
log.Debugf("[cmcc_cloud] pollToken attempt %d failed: %v", i+1, err)
continue
}
if resp.Data != nil && resp.Data.AccessToken != "" {
// 拿到 token！
d.Addition.AccessToken = resp.Data.AccessToken
d.Addition.OAuthUUID = ""
d.GetStorage().SetStatus("work")
op.MustSaveDriverStorage(d)
log.Infof("[cmcc_cloud] OAuth token obtained successfully")
return
}
log.Debugf("[cmcc_cloud] pollToken attempt %d: no token yet", i+1)
}
// 超时
d.GetStorage().SetStatus("授权超时(3分钟)，请在管理后台重新保存此驱动以重试")
op.MustSaveDriverStorage(d)
log.Warnf("[cmcc_cloud] pollToken timed out after %d attempts", maxTry)
}()
}

// ==================== AES 解密 ====================

// aesDecrypt AES CBC/Pkcs7 解密
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

// pkcs7Unpad PKCS7 去填充
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

// catalogToObj 将目录信息转换为 model.Obj
func catalogToObj(c CatalogInfo) model.Obj {
return &model.Object{
ID:       c.CatalogID,
Name:     c.CatalogName,
IsFolder: true,
Modified: parseTime(c.UpdateTime),
Ctime:    parseTime(c.CreateTime),
}
}

// contentToObj 将文件内容信息转换为 model.Obj
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

// parseTime 解析移动云盘时间格式
func parseTime(s string) time.Time {
if s == "" {
return time.Time{}
}
// 代理返回格式: "20260723111955" (yyyyMMddHHmmss)
for _, layout := range []string{
"20060102150405", // 代理 XML→JSON 格式
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

// parseSort 将用户配置的排序字段映射为 API 参数
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

// ==================== 错误处理 ====================

// isAPIError 判断 API 响应是否为错误
func isAPIError(resultCode string) bool {
return resultCode != "" && resultCode != "0" && !strings.HasPrefix(resultCode, "0")
}

// parseSize 解析文件大小（代理可能返回字符串或数字）
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
