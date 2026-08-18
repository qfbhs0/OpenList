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

// refreshToken 刷新 accesstoken（走代理，paramType=json）
func (d *CmccCloud) refreshToken() error {
	log.Debugf("[cmcc_cloud] refreshToken: attempting, appId=%s, tokenLen=%d", d.Addition.AppId, len(d.Addition.AccessToken))
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

		// 目录列表
		if result.GetDiskResult.CatalogList != nil {
			for _, catalog := range result.GetDiskResult.CatalogList.CatalogInfo {
				files = append(files, catalogToObj(catalog))
			}
		}
		// 文件列表
		if result.GetDiskResult.ContentList != nil {
			for _, content := range result.GetDiskResult.ContentList.ContentInfo {
				files = append(files, contentToObj(content))
			}
		}

		// 分页（代理返回字符串类型的 nodeCount）
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
	// API文档：downloadRequest 需要 contentID + OwnerMSISDN
	// OwnerMSISDN 在 getUserInfo 中获取，先用空字符串尝试
	req := DownloadReq{
		ContentID:   file.GetID(),
		OwnerMSISDN: "", // 留空，部分代理可能不需要
	}
	var result DownloadResp
	if err := d.postAPI(pathDownloadReq, req, "xml", &result); err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	// API文档：响应格式为 {"String": "<下载URL>"}
	downloadURL := result.String
	if downloadURL == "" {
		// 兼容：有些代理可能返回 downloadResult.downloadURL 格式
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

// ==================== MakeDir ====================

func (d *CmccCloud) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	req := CreateCatalogReq{
		ParentCatalogID: parentDir.GetID(),
		CatalogName:     dirName,
		CatalogType:     0,
	}
	var result CreateCatalogResp
	if err := d.postAPI(pathCreateCatalog, req, "xml", &result); err != nil {
		return nil, fmt.Errorf("mkdir failed: %w", err)
	}
	if isAPIError(result.ResultCode) {
		return nil, fmt.Errorf("mkdir api error: resultCode=%s", result.ResultCode)
	}

	catalogID := ""
	if result.CreateCatalogResult != nil {
		catalogID = result.CreateCatalogResult.CatalogID
	}
	return &model.Object{
		ID:       catalogID,
		Name:     dirName,
		IsFolder: true,
	}, nil
}

// ==================== Move ====================

func (d *CmccCloud) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	// API文档：moveContentCatalog，格式为 {newCatalogID, catalogInfoList:{ID:[]}} 或 {newCatalogID, contentInfoList:{ID:[]}}
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
		// API文档：ES 也不支持目录重命名，但尝试用 updateCatalogInfo
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

	// 文件重命名：API文档 updateContentInfo
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
	// API文档：delCatalogContent，格式为 {oprReason:"0", catalogIDs:{ID:[]}} 或 {oprReason:"0", contentIDs:{ID:[]}}
	req := DelReq{
		OprReason: "0", // 文档要求字符串
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

// ==================== Put (上传) ====================

func (d *CmccCloud) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	file, err := stream.CacheFullAndWriter(&up, nil)
	if err != nil {
		return nil, err
	}

	fileSize := stream.GetSize()

	// API文档：totalSize 为字符串，uploadContentList 包在对象里
	uploadReq := UploadReq{
		TotalSize: strconv.FormatInt(fileSize, 10),
		UploadContentList: UploadContentWrap{
			UploadContentInfo: []UploadContentInfo{
				{
					ContentName: stream.GetName(),
					ContentSize: strconv.FormatInt(fileSize, 10),
				},
			},
		},
	}

	var uploadResult UploadResp
	if err := d.postAPI(pathUploadReq, uploadReq, "xml", &uploadResult); err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	if isAPIError(uploadResult.ResultCode) {
		return nil, fmt.Errorf("upload api error: resultCode=%s", uploadResult.ResultCode)
	}
	if uploadResult.UploadResult == nil {
		return nil, errors.New("upload: empty uploadResult")
	}

	// API文档：uploadResult.redirectionUrl
	uploadURL := uploadResult.UploadResult.RedirectionUrl
	if uploadURL == "" {
		return nil, errors.New("empty upload url (redirectionUrl)")
	}
	uploadURL = strings.ReplaceAll(uploadURL, "&amp;", "&")

	reader := io.LimitReader(file, fileSize)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, reader)
	if err != nil {
		return nil, fmt.Errorf("create upload request failed: %w", err)
	}
	req2.ContentLength = fileSize
	req2.Header.Set("Content-Type", "application/octet-stream")
	req2.Header.Set("uploadtaskID", uploadResult.UploadResult.UploadTaskID)
	req2.Header.Set("contentSize", strconv.FormatInt(fileSize, 10))

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
	// 代理返回 MB 为单位的字符串
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
var _ driver.Remove = (*CmccCloud)(nil)