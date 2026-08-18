package cmcc_cloud

import "time"

// ==================== 通用响应 ====================
type APIResp struct {
Code    int    `json:"code"`
Message string `json:"message"`
}

// ==================== OAuth 相关 ====================
type AccessToken1Req struct {
UUID string `json:"uuid"`
}
type AccessToken1Resp struct {
Code    int              `json:"code"`
Message string           `json:"message"`
Data    *AccessTokenData `json:"data"`
}
type AccessTokenData struct {
AccessToken string `json:"accessToken"`
ExpiresIn   int    `json:"expiresIn"`
}
type RefreshTokenReq struct {
AppId        string `json:"appId"`
AppSecret    string `json:"appSecret"`
RefreshToken string `json:"refreshToken"`
}
type RefreshTokenResp struct {
Code    int              `json:"code"`
Message string           `json:"message"`
Data    *AccessTokenData `json:"data"`
}

// ==================== 用户信息 ====================
type UserInfoReq struct {
GetUserInfo struct {
QryType int `json:"qryType"`
} `json:"getUserInfo"`
}
type UserInfoResp struct {
VisualProUserInfo struct {
Msisdn string `json:"msisdn"`
} `json:"visualProUserInfo"`
ResultCode string `json:"resultCode"`
}

// ==================== 磁盘信息 ====================
type GetDiskInfoResp struct {
GetDiskInfoResult *GetDiskInfoResult `json:"getDiskInfoResult"`
ResultCode        string             `json:"resultCode"`
}
type GetDiskInfoResult struct {
DiskSize     string `json:"diskSize"`
FreeDiskSize string `json:"freeDiskSize"`
}

// ==================== 列目录 ====================
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
type GetDiskResp struct {
GetDiskResult *GetDiskResultInner `json:"getDiskResult"`
ResultCode    string              `json:"resultCode"`
}
type GetDiskResultInner struct {
CatalogList     *CatalogListObj `json:"catalogList"`
ContentList     *ContentListObj `json:"contentList"`
NodeCount       string          `json:"nodeCount"`
ParentCatalogID string          `json:"parentCatalogID"`
IsCompleted     string          `json:"isCompleted"`
}
type CatalogListObj struct {
CatalogInfo []CatalogInfo `json:"catalogInfo"`
Length      string        `json:"length"`
}
type ContentListObj struct {
ContentInfo []ContentInfo `json:"contentInfo"`
Length      string        `json:"length"`
}
type CatalogInfo struct {
CatalogID       string `json:"catalogID"`
CatalogName     string `json:"catalogName"`
CatalogType     string `json:"catalogType"`
ParentCatalogID string `json:"parentCatalogId"`
CreateTime      string `json:"createTime"`
UpdateTime      string `json:"updateTime"`
NodeCount       string `json:"nodeCount"`
Path            string `json:"path"`
Owner           string `json:"owner"`
}
type ContentInfo struct {
ContentID       string `json:"contentID"`
ContentName     string `json:"contentName"`
ContentType     string `json:"contentType"`
ContentSize     string `json:"contentSize"`
ParentCatalogID string `json:"parentCatalogId"`
CreateTime      string `json:"createTime"`
UpdateTime      string `json:"updateTime"`
Digest          string `json:"digest"`
Url             string `json:"url"`
}

// ==================== 下载 ====================
type DownloadReq struct {
ContentID   string `json:"contentID"`
OwnerMSISDN string `json:"OwnerMSISDN"`
}
type DownloadResp struct {
String     string `json:"String"`
ResultCode string `json:"resultCode"`
}

// ==================== 上传（两步法：先在ES浏览器目录上传，再移动到目标） ====================
// UploadReq pcUploadFileRequest 请求体
// 两步法时 ParentCatalogID 设为 ES浏览器目录ID
// 创建目录时 NewCatalogName 设为目录名，totalSize="0"，占位 contentSize="0"
type UploadReq struct {
TotalSize         string            `json:"totalSize"`
ParentCatalogID   string            `json:"parentCatalogID,omitempty"`
NewCatalogName    string            `json:"newCatalogName,omitempty"`
UploadContentList UploadContentWrap `json:"uploadContentList"`
}
type UploadContentWrap struct {
UploadContentInfo []UploadContentInfo `json:"uploadContentInfo"`
}
type UploadContentInfo struct {
ContentName string `json:"contentName"`
ContentSize string `json:"contentSize"`
}
// UploadResp pcUploadFileRequest 响应
type UploadResp struct {
UploadResult *UploadResultInner `json:"uploadResult"`
ResultCode   string            `json:"resultCode"`
}
type UploadResultInner struct {
RedirectionUrl   string            `json:"redirectionUrl"`
UploadTaskID     string            `json:"uploadTaskID"`
IsNeedUpload     string            `json:"isNeedUpload"`
NewContentIDList *NewContentIDWrap `json:"newContentIDList"`
}
// NewContentIDWrap 新建内容ID列表（代理 XML->JSON 把数组包在对象里）
type NewContentIDWrap struct {
NewContent []NewContentID `json:"newContent"`
}
type NewContentID struct {
ContentID string `json:"contentID"`
}

// ==================== 创建目录（代理禁止，走两步法） ====================
type CreateCatalogReq struct {
ParentCatalogID string `json:"parentCatalogID"`
CatalogName     string `json:"catalogName"`
CatalogType     int    `json:"catalogType"`
}
type CreateCatalogResp struct {
CreateCatalogResult *CreateCatalogResultInner `json:"createCatalogResult"`
ResultCode          string                   `json:"resultCode"`
}
type CreateCatalogResultInner struct {
CatalogID string `json:"catalogID"`
}

// ==================== 移动 ====================
type MoveReq struct {
NewCatalogID    string      `json:"newCatalogID"`
CatalogInfoList *IDListWrap `json:"catalogInfoList,omitempty"`
ContentInfoList *IDListWrap `json:"contentInfoList,omitempty"`
}
type IDListWrap struct {
ID []string `json:"ID"`
}
type MoveResp struct {
ResultCode string `json:"resultCode"`
}

// ==================== 重命名 ====================
type UpdateContentInfoReq struct {
ContentID   string `json:"contentID"`
ContentName string `json:"contentName"`
}
type UpdateContentInfoResp struct {
UpdateContentInfoRes *UpdateContentInfoResInner `json:"updateContentInfoRes"`
ResultCode          string                     `json:"resultCode"`
}
type UpdateContentInfoResInner struct {
ContentName string `json:"contentName"`
}
type UpdateCatalogInfoReq struct {
CatalogID   string `json:"catalogID"`
CatalogName string `json:"catalogName"`
}
type UpdateCatalogInfoResp struct {
ResultCode string `json:"resultCode"`
}

// ==================== 删除 ====================
type DelReq struct {
OprReason  string     `json:"oprReason"`
CatalogIDs *IDListWrap `json:"catalogIDs,omitempty"`
ContentIDs *IDListWrap `json:"contentIDs,omitempty"`
}
type DelResp struct {
ResultCode string `json:"resultCode"`
}

// ==================== 时间解析辅助 ====================
var _ time.Time
