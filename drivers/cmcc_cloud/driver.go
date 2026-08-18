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
	statusTimer *time.Timer
}

func (d *CmccCloud) Config() driver.Config {
	return config
}

func (d *CmccCloud) GetAddition() driver.Additional {
	return &d.Addition
}

// ==================== Init ====================
// Init 驱动初始化，处理 OAuth 授权闭环
//
// 流程：
//   1. 生成 DeviceId（如果为空）
//   2. 已有 AccessToken → 用 refreshToken 刷新
//   3. 无 AccessToken 但有 OAuthUUID → 先试 accessToken1 换 token，
//      没拿到则启动后台轮询 pollToken
//   4. 无 AccessToken 也无 OAuthUUID → 生成 UUID + 授权URL，显示给用户
func (d *CmccCloud) Init(ctx context.Context) error {
	// 生成 DeviceId（如果为空）
	if d.Addition.DeviceId == "" {
		d.Addition.DeviceId = generateDeviceId()
		op.MustSaveDriverStorage(d)
	}

	// ---- 已有 AccessToken，尝试刷新 ----
	if d.Addition.AccessToken != "" {
		if err := d.refreshToken(); err != nil {
			log.Warnf("[cmcc_cloud] refresh token failed: %v, need re-auth", err)
			// 刷新失败，清空 token，走重新授权流程
			d.Addition.AccessToken = ""
			d.Addition.OAuthUUID = ""
			op.MustSaveDriverStorage(d)
			// 继续往下走，不要 return error
		} else {
			return nil
		}
	}

	// ---- 无 AccessToken，但有 OAuthUUID（方案B：用户可能已授权） ----
	if d.Addition.OAuthUUID != "" {
		// 立刻试一次 accessToken1
		var resp AccessToken1Resp
		req := AccessToken1Req{UUID: d.Addition.OAuthUUID}
		if err := d.postJSON(baseURL+pathAccessToken1, req, &resp); err == nil {
			if resp.Data != nil && resp.Data.Accesstoken != "" {
				// 拿到了！
				d.Addition.AccessToken = resp.Data.Accesstoken
				d.Addition.OAuthUUID = ""
				op.MustSaveDriverStorage(d)
				log.Infof("[cmcc_cloud] token obtained via accessToken1 on Init")
				return nil
			}
		}
		// 没拿到，启动后台轮询（方案A）
		authURL := d.buildAuthURL(d.Addition.OAuthUUID)
		d.setTempStatus(fmt.Sprintf("请访问以下URL授权: %s", authURL))
		d.pollToken(d.Addition.OAuthUUID)
		return nil
	}

	// ---- 无 AccessToken 也无 OAuthUUID，首次授权 ----
	uuid := generateUUID(d.Addition.AppId)
	d.Addition.OAuthUUID = uuid
	op.MustSaveDriverStorage(d)

	authURL := d.buildAuthURL(uuid)
	d.setTempStatus(fmt.Sprintf("请访问以下URL授权: %s", authURL))

	// 启动后台轮询
	d.pollToken(uuid)
	return nil
}

// refreshToken 刷新 accesstoken
func (d *CmccCloud) refreshToken() error {
	log.Debugf("[cmcc_cloud] refreshToken: attempting refresh, appId=%s, tokenLen=%d", d.Addition.AppId, len(d.Addition.AccessToken))
	req := RefreshTokenReq{
		AppId:        d.Addition.AppId,
		AppSecret:    d.Addition.AppSecret,
		RefreshToken: d.Addition.AccessToken,
	}
	var resp RefreshTokenResp
	if err := d.postJSON(baseURL+pathRefreshToken, req, &resp); err != nil {
		log.Warnf("[cmcc_cloud] refreshToken: HTTP request failed: %v", err)
		return err
	}
	if resp.Data == nil || resp.Data.Accesstoken == "" {
		log.Warnf("[cmcc_cloud] refreshToken: API returned empty token, code=%d, msg=%s", resp.Code, resp.Message)
		return fmt.Errorf("refresh token failed: code=%d, msg=%s", resp.Code, resp.Message)
	}
	d.Addition.AccessToken = resp.Data.Accesstoken
	op.MustSaveDriverStorage(d)
	log.Infof("[cmcc_cloud] refreshToken: success, newTokenLen=%d", len(d.Addition.AccessToken))
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
		var result GetDiskResult
		if err := d.postJSON(baseURL+pathGetDisk, req, &result); err != nil {
			return nil, fmt.Errorf("list failed: %w", err)
		}
		if isAPIError(result.ResultCode) {
			return nil, fmt.Errorf("list api error: resultCode=%s", result.ResultCode)
		}
		for _, catalog := range result.CatalogList {
			files = append(files, catalogToObj(catalog))
		}
		for _, content := range result.ContentList {
			files = append(files, contentToObj(content))
		}
		totalNodes := result.NodeCount
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
		ContentID: file.GetID(),
	}
	var result DownloadResult
	if err := d.postJSON(baseURL+pathDownloadReq, req, &result); err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	if isAPIError(result.ResultCode) {
		return nil, fmt.Errorf("download api error: resultCode=%s", result.ResultCode)
	}
	downloadURL := result.DownloadURL
	if downloadURL == "" {
		return nil, errors.New("empty download url")
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

// ==================== MakeDir ====================
func (d *CmccCloud) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	req := CreateCatalogReq{
		ParentCatalogID: parentDir.GetID(),
		CatalogName:     dirName,
		CatalogType:     0,
	}
	var result CreateCatalogResult
	if err := d.postJSON(baseURL+pathCreateCatalog, req, &result); err != nil {
		return nil, fmt.Errorf("mkdir failed: %w", err)
	}
	if isAPIError(result.ResultCode) {
		return nil, fmt.Errorf("mkdir api error: resultCode=%s", result.ResultCode)
	}
	return &model.Object{
		ID:       result.CatalogID,
		Name:     dirName,
		IsFolder: true,
	}, nil
}

// ==================== Move ====================
func (d *CmccCloud) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	req := MoveCatalogContentReq{
		SourceCatalogID: "",
		DestCatalogID:   dstDir.GetID(),
	}
	if srcObj.IsDir() {
		req.CatalogIDs = []string{srcObj.GetID()}
	} else {
		req.ContentIDs = []string{srcObj.GetID()}
	}
	var result APIResp
	if err := d.postJSON(baseURL+pathMoveContent, req, &result); err != nil {
		return nil, fmt.Errorf("move failed: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("move api error: code=%d, msg=%s", result.Code, result.Message)
	}
	return srcObj, nil
}

// ==================== Rename ====================
func (d *CmccCloud) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	if srcObj.IsDir() {
		return nil, errs.NotSupport
	}
	req := UpdateContentInfoReq{
		ContentID:   srcObj.GetID(),
		ContentName: newName,
	}
	var result APIResp
	if err := d.postJSON(baseURL+pathUpdateContent, req, &result); err != nil {
		return nil, fmt.Errorf("rename failed: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("rename api error: code=%d, msg=%s", result.Code, result.Message)
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
	req := DelCatalogContentReq{
		OprReason: 0,
	}
	if obj.IsDir() {
		req.CatalogIDs = []string{obj.GetID()}
	} else {
		req.ContentIDs = []string{obj.GetID()}
	}
	var result APIResp
	if err := d.postJSON(baseURL+pathDelContent, req, &result); err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("remove api error: code=%d, msg=%s", result.Code, result.Message)
	}
	return nil
}

// ==================== Put (上传) ====================
func (d *CmccCloud) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	file, err := stream.CacheFullAndWriter(&up, nil)
	if err != nil {
		return nil, err
	}
	uploadReq := UploadReq{
		TotalSize: stream.GetSize(),
		UploadContentList: []UploadContentInfo{
			{
				ContentName: stream.GetName(),
				ContentSize: stream.GetSize(),
			},
		},
		ParentCatalogID: dstDir.GetID(),
		Operation:       0,
	}
	var uploadResult UploadResult
	if err := d.postJSON(baseURL+pathUploadReq, uploadReq, &uploadResult); err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	if isAPIError(uploadResult.ResultCode) {
		return nil, fmt.Errorf("upload api error: resultCode=%s", uploadResult.ResultCode)
	}
	uploadURL := uploadResult.UploadURL
	if uploadURL == "" {
		return nil, errors.New("empty upload url")
	}
	uploadURL = strings.ReplaceAll(uploadURL, "&amp;", "&")
	reader := io.LimitReader(file, stream.GetSize())
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, reader)
	if err != nil {
		return nil, fmt.Errorf("create upload request failed: %w", err)
	}
	req2.ContentLength = stream.GetSize()
	req2.Header.Set("Content-Type", "application/octet-stream")
	req2.Header.Set("uploadtaskID", uploadResult.UploadTaskID)
	req2.Header.Set("contentSize", strconv.FormatInt(stream.GetSize(), 10))
	resp, err := base.HttpClient.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("upload data failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}
	up(100)
	return &model.Object{
		Name:     stream.GetName(),
		Size:     stream.GetSize(),
		IsFolder: false,
		Modified: stream.ModTime(),
	}, nil
}

// ==================== GetDetails ====================
func (d *CmccCloud) GetDetails(ctx context.Context) (*model.StorageDetails, error) {
	var result GetDiskInfoResult
	if err := d.postJSON(baseURL+pathGetDiskInfo, nil, &result); err != nil {
		return nil, fmt.Errorf("get disk info failed: %w", err)
	}
	if isAPIError(result.ResultCode) {
		return nil, fmt.Errorf("get disk info error: resultCode=%s", result.ResultCode)
	}
	return &model.StorageDetails{
		DiskUsage: model.DiskUsage{
			TotalSpace: result.TotalSize,
			UsedSpace:  result.UsedSize,
		},
	}, nil
}

// ==================== 接口断言 ====================
var _ driver.Driver = (*CmccCloud)(nil)
var _ driver.MkdirResult = (*CmccCloud)(nil)
var _ driver.MoveResult = (*CmccCloud)(nil)
var _ driver.RenameResult = (*CmccCloud)(nil)
var _ driver.Remove = (*CmccCloud)(nil)
var _ driver.PutResult = (*CmccCloud)(nil)