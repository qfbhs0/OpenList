package cmcc_cloud

import "time"

// ==================== 通用响应 ====================

// APIResp 通用 API 响应
type APIResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIError API 错误响应
type APIError struct {
	ResultCode string `json:"resultCode"`
	Message    string `json:"message"`
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
	Accesstoken string `json:"accesstoken"`
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

// UserInfoResp 获取用户信息响应
type UserInfoResp struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *UserInfoData `json:"data"`
}

// UserInfoData 用户信息数据
type UserInfoData struct {
	Phone     string `json:"phone"`     // AES 加密
	NickName  string `json:"nickName"`  // AES 加密
	AvatarURL string `json:"avatarUrl"`
}

// ==================== 磁盘信息 ====================

// GetDiskInfoResult 获取磁盘信息结果
type GetDiskInfoResult struct {
	ResultCode string `json:"resultCode"`
	Message    string `json:"message"`
	TotalSize  int64  `json:"totalSize"`
	UsedSize   int64  `json:"usedSize"`
}

// ==================== 列目录 ====================

// GetDiskReq 列目录请求
type GetDiskReq struct {
	CatalogID       string `json:"catalogID"`
	FilterType      int    `json:"filterType"`      // 0=全部, 1=仅目录, 2=仅文件
	CatalogSortType int    `json:"catalogSortType"` // 0=更新时间, 1=名称, 2=类型, 3=大小
	ContentType     int    `json:"contentType"`     // 0=全部
	ContentSortType int    `json:"contentSortType"` // 同 catalogSortType
	SortDirection   int    `json:"sortDirection"`   // 0=正序, 1=倒序
	StartNumber     int    `json:"startNumber"`     // 起始序号(从1开始)
	EndNumber       int    `json:"endNumber"`       // 结束序号
	CatalogType     int    `json:"catalogType"`     // -1=所有类型
}

// GetDiskResult 列目录结果
type GetDiskResult struct {
	ResultCode string        `json:"resultCode"`
	Message    string        `json:"message"`
	CatalogList []CatalogInfo `json:"catalogList"`
	ContentList []ContentInfo `json:"contentList"`
	NodeCount   int          `json:"nodeCount"` // 总节点数(用于分页)
}

// CatalogInfo 目录信息
type CatalogInfo struct {
	CatalogID   string `json:"catalogID"`
	CatalogName string `json:"catalogName"`
	CatalogType int    `json:"catalogType"`
	ParentCatalogID string `json:"parentCatalogID"`
	CreateTime  string `json:"createTime"`
	UpdateTime  string `json:"updateTime"`
	NodeCount   int    `json:"nodeCount"`
}

// ContentInfo 文件内容信息
type ContentInfo struct {
	ContentID    string `json:"contentID"`
	ContentName  string `json:"contentName"`
	ContentType  int    `json:"contentType"`
	ContentSize  int64  `json:"contentSize"`
	ParentCatalogID string `json:"parentCatalogID"`
	CreateTime   string `json:"createTime"`
	UpdateTime   string `json:"updateTime"`
	Digest       string `json:"digest"`
	Url          string `json:"url"`
}

// ==================== 下载 ====================

// DownloadReq 下载请求
type DownloadReq struct {
	ContentID string `json:"contentID"`
}

// DownloadResult 下载结果
type DownloadResult struct {
	ResultCode  string `json:"resultCode"`
	Message     string `json:"message"`
	DownloadURL string `json:"downloadURL"`
}

// ==================== 上传 ====================

// UploadReq 上传请求
type UploadReq struct {
	TotalSize        int64              `json:"totalSize"`
	UploadContentList []UploadContentInfo `json:"uploadContentList"`
	ParentCatalogID  string             `json:"parentCatalogID"`
	Operation        int                `json:"operation"` // 0=普通上传
}

// UploadContentInfo 上传文件信息
type UploadContentInfo struct {
	ContentName string `json:"contentName"`
	ContentSize int64  `json:"contentSize"`
	Digest      string `json:"digest,omitempty"` // 可选 MD5
}

// UploadResult 上传结果
type UploadResult struct {
	ResultCode   string `json:"resultCode"`
	Message      string `json:"message"`
	UploadURL    string `json:"uploadURL"`
	UploadTaskID string `json:"uploadTaskID"`
}

// ==================== 创建目录 ====================

// CreateCatalogReq 创建目录请求
type CreateCatalogReq struct {
	ParentCatalogID string `json:"parentCatalogID"`
	CatalogName     string `json:"catalogName"`
	CatalogType     int    `json:"catalogType"` // 0=普通目录
}

// CreateCatalogResult 创建目录结果
type CreateCatalogResult struct {
	ResultCode string `json:"resultCode"`
	Message    string `json:"message"`
	CatalogID  string `json:"catalogID"`
}

// ==================== 移动 ====================

// MoveCatalogContentReq 移动内容请求
type MoveCatalogContentReq struct {
	SourceCatalogID string   `json:"sourceCatalogID,omitempty"`
	DestCatalogID   string   `json:"destCatalogID"`
	CatalogIDs      []string `json:"catalogIDs,omitempty"`  // 移动目录的ID列表
	ContentIDs      []string `json:"contentIDs,omitempty"`  // 移动文件的ID列表
}

// ==================== 重命名/更新 ====================

// UpdateContentInfoReq 更新文件信息请求（用于文件重命名）
type UpdateContentInfoReq struct {
	ContentID   string `json:"contentID"`
	ContentName string `json:"contentName"`
}

// UpdateCatalogInfoReq 更新目录信息请求（用于目录重命名，需 XML 格式）
type UpdateCatalogInfoReq struct {
	CatalogID   string `json:"catalogID"`
	CatalogName string `json:"catalogName"`
}

// ==================== 删除 ====================

// DelCatalogContentReq 删除内容请求
type DelCatalogContentReq struct {
	CatalogIDs []string `json:"catalogIDs,omitempty"`  // 删除目录的ID列表
	ContentIDs []string `json:"contentIDs,omitempty"`  // 删除文件的ID列表
	OprReason  int      `json:"oprReason"`             // 0=普通删除
}

// ==================== 时间解析辅助（供 types 内部使用） ====================

// _ 确保时间包被导入
var _ time.Time
