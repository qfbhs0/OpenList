package cmcc_cloud

import "time"

// ==================== 通用响应 ====================

// APIResp 通用 API 响应（代理返回的 code/message 格式）
type APIResp struct {
Code    int    `json:"code"`
Message string `json:"message"`
}

// APIError API 错误响应（代理返回的 resultCode/desc 格式）
type APIError struct {
ResultCode string `json:"resultCode"`
Message    string `json:"desc"` // 代理用 "desc" 而非 "message"
}

// ==================== OAuth 相关 ====================

// AccessToken1Req 用 UUID 换取 accesstoken 请求
type AccessToken1Req struct {
UUID string `json:"uuid"`
}

// AccessToken1Resp 用 UUID 换取 accesstoken 响应（代理格式）
type AccessToken1Resp struct {
Code    int              `json:"code"`
Message string           `json:"message"`
Data    *AccessTokenData `json:"data"`
}

// AccessTokenData accesstoken 数据
// 注意：代理返回 accessToken（驼峰），非 accesstoken
type AccessTokenData struct {
AccessToken string `json:"accessToken"`
ExpiresIn   int    `json:"expiresIn"`
}

// RefreshTokenReq 刷新 token 请求
type RefreshTokenReq struct {
AppId        string `json:"appId"`
AppSecret    string `json:"appSecret"`
RefreshToken string `json:"refreshToken"`
}

// RefreshTokenResp 刷新 token 响应（代理格式）
type RefreshTokenResp struct {
Code    int              `json:"code"`
Message string           `json:"message"`
Data    *AccessTokenData `json:"data"`
}

// ==================== 用户信息 ====================

type UserInfoResp struct {
Code    int          `json:"code"`
Message string       `json:"message"`
Data    *UserInfoData `json:"data"`
}

type UserInfoData struct {
Phone     string `json:"phone"`
NickName  string `json:"nickName"`
AvatarURL string `json:"avatarUrl"`
}

// ==================== 磁盘信息 ====================

// GetDiskInfoResp 获取磁盘信息响应（代理 XML→JSON 格式）
type GetDiskInfoResp struct {
GetDiskInfoResult *GetDiskInfoResult `json:"getDiskInfoResult"`
ResultCode        string             `json:"resultCode"`
}

type GetDiskInfoResult struct {
DiskSize    string `json:"diskSize"`    // 单位:MB，字符串类型
FreeDiskSize string `json:"freeDiskSize"` // 单位:MB，字符串类型
}

// ==================== 列目录 ====================

// GetDiskReq 列目录请求
type GetDiskReq struct {
CatalogID       string `json:"catalogID"`
FilterType      int    `json:"filterType"`
CatalogSortType int    `json:"catalogSortType"`
ContentType     int    `json:"contentType"`
ContentSortType int    `json:"contentSortType"`
SortDirection   int    `json:"sortDirection"`
StartNumber     int    `json:"startNumber"`
EndNumber       int    `json:"endNumber"`
CatalogType     int    `json:"catalogType"`
}

// GetDiskResp 列目录响应（代理 XML→JSON 格式）
// 代理把 XML 响应转为 JSON 后，结构有包裹层
type GetDiskResp struct {
GetDiskResult *GetDiskResultInner `json:"getDiskResult"`
ResultCode    string              `json:"resultCode"`
}

// GetDiskResultInner 内层结果
type GetDiskResultInner struct {
CatalogList    *CatalogListObj    `json:"catalogList"`
ContentList    *ContentListObj    `json:"contentList"`
NodeCount      string            `json:"nodeCount"` // 代理返回字符串
ParentCatalogID string           `json:"parentCatalogID"`
IsCompleted    string            `json:"isCompleted"`
}

// CatalogListObj 目录列表（代理 XML→JSON 把数组包在对象里）
type CatalogListObj struct {
CatalogInfo []CatalogInfo `json:"catalogInfo"`
Length      string        `json:"length"`
}

// ContentListObj 文件列表（代理 XML→JSON 把数组包在对象里）
type ContentListObj struct {
ContentInfo []ContentInfo `json:"contentInfo"`
Length      string        `json:"length"`
}

// CatalogInfo 目录信息
type CatalogInfo struct {
CatalogID       string `json:"catalogID"`
CatalogName     string `json:"catalogName"`
CatalogType     string `json:"catalogType"`     // 代理返回字符串
ParentCatalogID string `json:"parentCatalogId"` // 注意小写d
CreateTime      string `json:"createTime"`
UpdateTime      string `json:"updateTime"`
NodeCount       string `json:"nodeCount"`
Path            string `json:"path"`
Owner           string `json:"owner"`
}

// ContentInfo 文件内容信息
type ContentInfo struct {
ContentID       string `json:"contentID"`
ContentName     string `json:"contentName"`
ContentType     string `json:"contentType"`  // 代理返回字符串
ContentSize     int64  `json:"contentSize"`  // 这个可能仍是数字
ParentCatalogID string `json:"parentCatalogId"`
CreateTime      string `json:"createTime"`
UpdateTime      string `json:"updateTime"`
Digest          string `json:"digest"`
Url             string `json:"url"`
}

// ==================== 下载 ====================

type DownloadReq struct {
ContentID string `json:"contentID"`
}

// DownloadResp 下载响应（代理 XML→JSON 格式）
type DownloadResp struct {
DownloadResult *DownloadResultInner `json:"downloadResult"`
ResultCode     string              `json:"resultCode"`
}

type DownloadResultInner struct {
DownloadURL string `json:"downloadURL"`
}

// ==================== 上传 ====================

type UploadReq struct {
TotalSize         int64              `json:"totalSize"`
UploadContentList []UploadContentInfo `json:"uploadContentList"`
ParentCatalogID   string             `json:"parentCatalogID"`
Operation         int                `json:"operation"`
}

type UploadContentInfo struct {
ContentName string `json:"contentName"`
ContentSize int64  `json:"contentSize"`
Digest      string `json:"digest,omitempty"`
}

// UploadResp 上传响应（代理 XML→JSON 格式）
type UploadResp struct {
UploadResult *UploadResultInner `json:"uploadResult"`
ResultCode   string            `json:"resultCode"`
}

type UploadResultInner struct {
UploadURL    string `json:"uploadURL"`
UploadTaskID string `json:"uploadTaskID"`
}

// ==================== 创建目录 ====================

type CreateCatalogReq struct {
ParentCatalogID string `json:"parentCatalogID"`
CatalogName     string `json:"catalogName"`
CatalogType     int    `json:"catalogType"`
}

// CreateCatalogResp 创建目录响应（代理 XML→JSON 格式）
type CreateCatalogResp struct {
CreateCatalogResult *CreateCatalogResultInner `json:"createCatalogResult"`
ResultCode          string                   `json:"resultCode"`
}

type CreateCatalogResultInner struct {
CatalogID string `json:"catalogID"`
}

// ==================== 移动 ====================

type MoveCatalogContentReq struct {
SourceCatalogID string   `json:"sourceCatalogID,omitempty"`
DestCatalogID   string   `json:"destCatalogID"`
CatalogIDs      []string `json:"catalogIDs,omitempty"`
ContentIDs      []string `json:"contentIDs,omitempty"`
}

// MoveResp 移动响应（代理 XML→JSON 格式）
type MoveResp struct {
ResultCode string `json:"resultCode"`
}

// ==================== 重命名/更新 ====================

type UpdateContentInfoReq struct {
ContentID   string `json:"contentID"`
ContentName string `json:"contentName"`
}

type UpdateCatalogInfoReq struct {
CatalogID   string `json:"catalogID"`
CatalogName string `json:"catalogName"`
}

// ==================== 删除 ====================

type DelCatalogContentReq struct {
CatalogIDs []string `json:"catalogIDs,omitempty"`
ContentIDs []string `json:"contentIDs,omitempty"`
OprReason  int      `json:"oprReason"`
}

// DelResp 删除响应（代理 XML→JSON 格式）
type DelResp struct {
ResultCode string `json:"resultCode"`
}

// ==================== 时间解析辅助 ====================
var _ time.Time
