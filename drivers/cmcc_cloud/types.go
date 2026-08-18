package cmcc_cloud

import "time"

// ==================== 通用响应 ====================

// APIResp 通用 API 响应（OAuth/Token 等返回的 code/message 格式）
type APIResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ==================== OAuth 相关 ====================

// AccessToken1Req 用 UUID 换取 accesstoken 请求
type AccessToken1Req struct {
	UUID string `json:"uuid"`
}

// AccessToken1Resp 用 UUID 换取 accesstoken 响应
type AccessToken1Resp struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *AccessTokenData `json:"data"`
}

// AccessTokenData accesstoken 数据
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

// RefreshTokenResp 刷新 token 响应
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

// GetDiskInfoResp 获取磁盘信息响应
type GetDiskInfoResp struct {
	GetDiskInfoResult *GetDiskInfoResult `json:"getDiskInfoResult"`
	ResultCode        string             `json:"resultCode"`
}

type GetDiskInfoResult struct {
	DiskSize     string `json:"diskSize"`     // 单位:MB，字符串
	FreeDiskSize string `json:"freeDiskSize"` // 单位:MB，字符串
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
type GetDiskResp struct {
	GetDiskResult *GetDiskResultInner `json:"getDiskResult"`
	ResultCode    string              `json:"resultCode"`
}

// GetDiskResultInner 内层结果
type GetDiskResultInner struct {
	CatalogList     *CatalogListObj `json:"catalogList"`
	ContentList     *ContentListObj `json:"contentList"`
	NodeCount       string          `json:"nodeCount"`
	ParentCatalogID string          `json:"parentCatalogID"`
	IsCompleted     string          `json:"isCompleted"`
}

// CatalogListObj 目录列表（代理把数组包在对象里）
type CatalogListObj struct {
	CatalogInfo []CatalogInfo `json:"catalogInfo"`
	Length      string        `json:"length"`
}

// ContentListObj 文件列表（代理把数组包在对象里）
type ContentListObj struct {
	ContentInfo []ContentInfo `json:"contentInfo"`
	Length      string        `json:"length"`
}

// CatalogInfo 目录信息
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

// ContentInfo 文件内容信息
type ContentInfo struct {
	ContentID       string `json:"contentID"`
	ContentName     string `json:"contentName"`
	ContentType     string `json:"contentType"`
	ContentSize     string `json:"contentSize"` // 代理返回字符串
	ParentCatalogID string `json:"parentCatalogId"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
	Digest          string `json:"digest"`
	Url             string `json:"url"`
}

// ==================== 下载 ====================

// DownloadReq 下载请求（API文档要求含 OwnerMSISDN）
type DownloadReq struct {
	ContentID   string `json:"contentID"`
	OwnerMSISDN string `json:"OwnerMSISDN"`
}

// DownloadResp 下载响应（API文档：{"String": "<下载URL>"})
type DownloadResp struct {
	String     string `json:"String"`      // 下载URL
	ResultCode string `json:"resultCode"`  // 可能也包含
}

// ==================== 上传 ====================

// UploadReq 上传请求（API文档格式：totalSize 为字符串，uploadContentList 包在对象里）
type UploadReq struct {
	TotalSize         string             `json:"totalSize"`
	UploadContentList UploadContentWrap  `json:"uploadContentList"`
}

// UploadContentWrap 上传内容列表包裹（代理 XML→JSON 把数组包在对象里）
type UploadContentWrap struct {
	UploadContentInfo []UploadContentInfo `json:"uploadContentInfo"`
}

type UploadContentInfo struct {
	ContentName string `json:"contentName"`
	ContentSize string `json:"contentSize"` // 代理用字符串
}

// UploadResp 上传响应（API文档：uploadResult.redirectionUrl）
type UploadResp struct {
	UploadResult *UploadResultInner `json:"uploadResult"`
	ResultCode   string            `json:"resultCode"`
}

type UploadResultInner struct {
	RedirectionUrl string `json:"redirectionUrl"` // API文档用 redirectionUrl
	UploadTaskID   string `json:"uploadTaskID"`
}

// ==================== 创建目录 ====================

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

// ==================== 移动（API文档：moveContentCatalog） ====================

// MoveReq 移动请求（API文档格式：newCatalogID + catalogInfoList/contentInfoList 包在对象里）
type MoveReq struct {
	NewCatalogID     string          `json:"newCatalogID"`
	CatalogInfoList  *IDListWrap     `json:"catalogInfoList,omitempty"`
	ContentInfoList  *IDListWrap     `json:"contentInfoList,omitempty"`
}

// IDListWrap ID列表包裹（代理 XML→JSON 格式：{ID: [...]}）
type IDListWrap struct {
	ID []string `json:"ID"`
}

// MoveResp 移动响应
type MoveResp struct {
	ResultCode string `json:"resultCode"`
}

// ==================== 重命名/更新 ====================

// UpdateContentInfoReq 重命名文件请求
type UpdateContentInfoReq struct {
	ContentID   string `json:"contentID"`
	ContentName string `json:"contentName"`
}

// UpdateContentInfoResp 重命名文件响应（API文档：updateContentInfoRes.contentName）
type UpdateContentInfoResp struct {
	UpdateContentInfoRes *UpdateContentInfoResInner `json:"updateContentInfoRes"`
	ResultCode          string                     `json:"resultCode"`
}

type UpdateContentInfoResInner struct {
	ContentName string `json:"contentName"`
}

// UpdateCatalogInfoReq 重命名目录请求
type UpdateCatalogInfoReq struct {
	CatalogID   string `json:"catalogID"`
	CatalogName string `json:"catalogName"`
}

type UpdateCatalogInfoResp struct {
	ResultCode string `json:"resultCode"`
}

// ==================== 删除（API文档格式：oprReason 为字符串，ID 列表包在对象里） ====================

// DelReq 删除请求
type DelReq struct {
	OprReason  string     `json:"oprReason"`
	CatalogIDs *IDListWrap `json:"catalogIDs,omitempty"`
	ContentIDs *IDListWrap `json:"contentIDs,omitempty"`
}

// DelResp 删除响应
type DelResp struct {
	ResultCode string `json:"resultCode"`
}

// ==================== 时间解析辅助 ====================

var _ time.Time
