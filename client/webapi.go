package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/ipfs/go-cid"
)

const isAssetAlreadyExist = 1017

const (
	AssetTransferTypeUpload   = "upload"
	AssetTransferTypeDownload = "download"

	AssetTransferStateSuccess = 1
	AssetTransferStateFailed  = 2
)

// Webserver defines the interface for the scheduler.
type Webserver interface {
	// AuthVerify checks whether the specified token is valid and returns the list of permissions associated with it.
	// AuthVerify(ctx context.Context, token string) (*JWTPayload, error)
	// GetVipInfo() (string, error)
	GetVipInfo(ctx context.Context) (*VipInfo, error)
	// ListAreaIDs list all area id
	ListAreaIDs(ctx context.Context) ([]string, error)
	// CreateAsset creates an asset with car CID, car name, and car size.
	CreateAsset(ctx context.Context, req *CreateAssetReq) (*CreateAssetRsp, error)
	// DeleteAsset deletes the asset of the user.
	DeleteAsset(ctx context.Context, userID, assetCID string) error
	// ShareAsset shares the assets of the user.
	ShareAsset(ctx context.Context, userID, areaID, assetCID string, needTrace bool) (*ShareAssetResult, error)
	// GetCandidateIPs retrieves information about candidate IPs.
	GetCandidateIPs(ctx context.Context) ([]*CandidateIPInfo, error)
	// ListAssets lists the assets of the user.
	ListAssets(ctx context.Context, parent, pageSize, page int, cid string, folderID int) (*ListAssetRecordRsp, error)
	// RenameAsset Rename a specific file
	RenameAsset(ctx context.Context, assetCID string, newName string) error

	// CreateGroup create Asset group
	CreateGroup(ctx context.Context, name string, parent int) (*AssetGroup, error)
	// ListGroups get groups on parent group
	ListGroups(ctx context.Context, parent, pageSize, page int) (*ListAssetGroupRsp, error)
	// ListAssetSummary list Asset and group
	ListAssetSummary(ctx context.Context, userID string, parent, limit, offset int) (*ListAssetSummaryRsp, error)
	// DeleteGroup delete a group
	DeleteGroup(ctx context.Context, userID string, groupID int) error
	// RenameGroup rename group
	RenameGroup(ctx context.Context, userID, newName string, groupID int) error
	// MoveAssetToGroup move a asset to group
	MoveAssetToGroup(ctx context.Context, userID, cid string, groupID int) error
	// MoveAssetGroup move a asset group
	MoveAssetGroup(ctx context.Context, userID string, groupID, targetGroupID int) error
	// GetAPPKeyPermissions get the permissions of user app key
	GetAPPKeyPermissions(ctx context.Context, userID, keyName string) ([]string, error)
	// GetNodeUploadInfo
	GetNodeUploadInfo(ctx context.Context, userID, area string, urlMode bool) (*UploadInfo, error)
	// AssetTransferReport
	AssetTransferReport(ctx context.Context, req AssetTransferReq) error

	// GetUserStorage
	GetUserStorage(ctx context.Context) (*UserStorageInfo, error)

	// GetAssetCount
	GetAssetCount(ctx context.Context) (*AssetCountInfo, error)
}

var _ Webserver = (*webserver)(nil)

// NewWebserver creates a new Scheduler instance with the specified URL, headers, and options.
func NewWebserver(url string, apiKey, token string) Webserver {
	return &webserver{url: url, apiKey: apiKey, token: token, client: http.DefaultClient}
}

type webserver struct {
	// client *Client
	url    string
	client *http.Client

	apiKey string
	token  string
}

func (s *webserver) GetVipInfo(ctx context.Context) (*VipInfo, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_vip_info", s.url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	vipInfo := &VipInfo{}
	err = interfaceToStruct(ret.Data, vipInfo)
	if err != nil {
		return nil, err
	}

	return vipInfo, nil
}

type ListAreaID struct {
	AreaMaps []AreaInfo `json:"area_maps"`
	List     []string   `json:"list"`
}

type AreaInfo struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s *webserver) ListAreaIDs(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_area_id", s.url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	var listAreas = &ListAreaID{}
	err = interfaceToStruct(ret.Data, listAreas)
	if err != nil {
		return nil, err
	}

	// fmt.Println("body ", string(body))
	return listAreas.List, nil
}

type webCreateAssetReq struct {
	AssetName string   `json:"asset_name"`
	AssetCID  string   `json:"asset_cid"`
	AreaID    []string `json:"area_id"`
	NodeID    string   `json:"node_id"`
	AssetType string   `json:"asset_type"`
	AssetSize int64    `json:"asset_size"`
	GroupID   int64    `json:"group_id"`
	Encrypted bool     `json:"encrypted"`
	NeedTrace bool     `json:"need_trace"`
}

// CreateUserAsset creates a new user asset.
func (s *webserver) CreateAsset(ctx context.Context, caReq *CreateAssetReq) (*CreateAssetRsp, error) {
	uploadUrl := fmt.Sprintf("%s/api/v1/storage/create_asset", s.url)
	// uploadUrl := fmt.Sprintf("%s/api/v1/storage/create_asset?area_id=%s&asset_name=%s&asset_cid=%s&node_id=%s&asset_type=%s&asset_size=%d&group_id=%d",
	// 	s.url, caReq.AreaID, neturl.QueryEscape(caReq.AssetName), caReq.AssetCID, caReq.NodeID, caReq.AssetType, caReq.AssetSize, caReq.GroupID)

	postData := webCreateAssetReq{
		AssetName: caReq.AssetName,
		AssetCID:  caReq.AssetCID,
		AreaID:    caReq.AreaIDs,
		NodeID:    caReq.NodeID,
		AssetType: caReq.AssetType,
		AssetSize: caReq.AssetSize,
		GroupID:   int64(caReq.GroupID),
	}

	jsonBytes, err := json.Marshal(postData)
	if err != nil {
		return nil, err
	}
	// fmt.Println("url: ", uploadUrl, "data: ", string(jsonBytes))

	req, err := http.NewRequestWithContext(ctx, "POST", uploadUrl, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		if ret.Err == isAssetAlreadyExist {
			return &CreateAssetRsp{IsAlreadyExist: true, Endpoints: nil}, nil
		}
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	endpoints := make([]*Endpoint, 0)
	err = interfaceToStruct(ret.Data, &endpoints)
	if err != nil {
		return nil, err
	}
	// fmt.Println("body ", string(body))
	return &CreateAssetRsp{IsAlreadyExist: len(endpoints) == 0, Endpoints: endpoints}, nil
}

// DeleteAsset deletes a user asset.
func (s *webserver) DeleteAsset(ctx context.Context, userID, assetCID string) error {
	url := fmt.Sprintf("%s/api/v1/storage/delete_asset?user_id=%s&asset_cid=%s", s.url, userID, assetCID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return err
	}

	if ret.Code != 0 {
		return fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	return nil
}

// ShareAsset shares user assets.
func (s *webserver) ShareAsset(ctx context.Context, userID, areaID, assetCID string, needTrace bool) (*ShareAssetResult, error) {
	// url := fmt.Sprintf("%s/api/v1/storage/share_asset?user_id=%s&area_id=%s&asset_cid=%s&need_trace=true", s.url, userID, areaID, assetCID)
	url := fmt.Sprintf("%s/api/v1/storage/share_asset?area_id=%s&asset_cid=%s", s.url, areaID, assetCID)
	if needTrace {
		url += "&need_trace=true"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	// log.Printf("url:%v apikey:%v token:%v", url, s.apiKey, s.token)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	rsp.Body = io.NopCloser(bytes.NewBuffer(body))

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	// requestRaw, _ := httputil.DumpRequest(req, true)
	// responseRaw, _ := httputil.DumpResponse(rsp, true)

	// log.Printf("ShareAsset DUMP:\n request: %s\nresponse: %s\n", string(requestRaw), string(responseRaw))

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	result := &ShareAssetResult{}
	err = interfaceToStruct(ret.Data, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetCandidateIPs retrieves candidate IPs.
func (s *webserver) GetCandidateIPs(ctx context.Context) ([]*CandidateIPInfo, error) {
	return nil, nil
}

// ListAssets lists user assets.
func (s *webserver) ListAssets(ctx context.Context, parent, pageSize, page int, cid string, folderID int) (*ListAssetRecordRsp, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_asset_group_list?parent=%d&page_size=%d&page=%d", s.url, parent, pageSize, page)
	if cid != "" {
		url += fmt.Sprintf("&cid=%s", cid)
	}
	if folderID > 0 {
		url += fmt.Sprintf("&groupid=%d", folderID)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	type Object struct {
		AssetOverview *AssetOverview `json:"AssetOverview"`
	}

	data := struct {
		List  []*Object `json:"list"`
		Total int       `json:"total"`
	}{}

	err = interfaceToStruct(ret.Data, &data)
	if err != nil {
		return nil, err
	}

	assetOverviews := make([]*AssetOverview, 0)
	for _, obj := range data.List {
		if obj.AssetOverview == nil {
			continue
		}
		assetOverviews = append(assetOverviews, obj.AssetOverview)
	}
	// fmt.Println("body ", string(body))
	return &ListAssetRecordRsp{Total: data.Total, AssetOverviews: assetOverviews}, nil
}

// RenameAssetReq 重命名文件请求
type RenameAssetReq struct {
	AssetCID string `json:"asset_cid"`
	NewName  string `json:"new_name"`
	// GroupID  int    `json:"group_id"`
}

// RenameAsset Rename a specific file
func (s *webserver) RenameAsset(ctx context.Context, assetCID string, newName string) error {
	url := fmt.Sprintf("%s/api/v1/storage/rename_asset", s.url)

	renameAssetReq := &RenameAssetReq{
		AssetCID: assetCID,
		NewName:  newName,
	}

	jsonBytes, err := json.Marshal(renameAssetReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf("status code %d %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return err
	}

	if ret.Code != 0 {
		return fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}
	return nil
}

// CreateGroup create a group
func (s *webserver) CreateGroup(ctx context.Context, name string, parent int) (*AssetGroup, error) {
	url := fmt.Sprintf("%s/api/v1/storage/create_group?&name=%s&parent=%d", s.url, name, parent)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	data := struct {
		Group *AssetGroup `json:"group"`
	}{}
	err = interfaceToStruct(ret.Data, &data)
	if err != nil {
		return nil, err
	}

	return data.Group, nil
}

// ListGroups list Asset group
func (s *webserver) ListGroups(ctx context.Context, parent, pageSize, page int) (*ListAssetGroupRsp, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_groups?parent=%d&page_size=%d&page=%d", s.url, parent, pageSize, page)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	listAssetGroupRsp := &ListAssetGroupRsp{}
	err = interfaceToStruct(ret.Data, listAssetGroupRsp)
	if err != nil {
		return nil, err
	}

	return listAssetGroupRsp, nil
}

// ListAssetSummary list Asset and group
func (s *webserver) ListAssetSummary(ctx context.Context, userID string, parent, limit, offset int) (*ListAssetSummaryRsp, error) {
	return nil, nil
}

// DeleteGroup delete a group
func (s *webserver) DeleteGroup(ctx context.Context, userID string, gid int) error {
	url := fmt.Sprintf("%s/api/v1/storage/delete_group?user_id=%s&group_id=%d", s.url, userID, gid)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf("status code %d %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return err
	}

	if ret.Code != 0 {
		return fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}
	return nil
}

// RenameGroup rename group
func (s *webserver) RenameGroup(ctx context.Context, userID, newName string, groupID int) error {
	return fmt.Errorf("not implemnet")
}

// MoveAssetToGroup move a asset to group
func (s *webserver) MoveAssetToGroup(ctx context.Context, userID, cid string, groupID int) error {
	return fmt.Errorf("not implemnet")
}

// MoveAssetGroup move a asset group
func (s *webserver) MoveAssetGroup(ctx context.Context, userID string, groupID, targetGroupID int) error {
	return fmt.Errorf("not implemnet")
}

// GetAPPKeyPermissions get the permissions of user app key
func (s *webserver) GetAPPKeyPermissions(ctx context.Context, userID, keyName string) ([]string, error) {
	return nil, nil
}

// GetNodeUploadInfo
func (s *webserver) GetNodeUploadInfo(ctx context.Context, userID, area string, urlMode bool) (*UploadInfo, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_upload_info?encrypted=false&need_trace=true", s.url)
	if urlMode {
		url += "&urlMode=true"
	}
	if area != "" {
		url += fmt.Sprintf("&area_id=%s", area)
	}

	// fmt.Println("GetUploadInfo url: ", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	uploadNodes := &UploadInfo{}
	err = interfaceToStruct(ret.Data, uploadNodes)
	if err != nil {
		return nil, err
	}

	return uploadNodes, nil
}

// AssetTransferReport
func (s *webserver) AssetTransferReport(ctx context.Context, req AssetTransferReq) error {
	reportUrl := fmt.Sprintf("%s/api/v1/storage/transfer/report", s.url)

	if req.Cid != "" {
		hash, err := CIDToHash(req.Cid)
		if err != nil {
			return err
		}

		req.Hash = hash
	}

	// postData := AssetTransferReq{
	// 	Cid:          cid,
	// 	Hash:         hash,
	// 	CostMs:       cost,
	// 	TotalSize:    totalSize,
	// 	Succeed:      succeed,
	// 	TransferType: transferType,
	// }

	if req.State == AssetTransferStateSuccess {
		// bytes per second
		req.Rate = req.TotalSize / req.CostMs * 1000
	}

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, "POST", reportUrl, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apikey", s.apiKey)

	rsp, err := s.client.Do(request)
	if err != nil {
		return err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return fmt.Errorf("status code %d, %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return err
	}

	if ret.Code != 0 {
		return fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	return nil
}

// GetUserStorage
func (s *webserver) GetUserStorage(ctx context.Context) (*UserStorageInfo, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_storage_size", s.url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	storageInfo := &UserStorageInfo{}
	err = interfaceToStruct(ret.Data, storageInfo)
	if err != nil {
		return nil, err
	}

	return storageInfo, nil
}

// GetAssetCount
func (s *webserver) GetAssetCount(ctx context.Context) (*AssetCountInfo, error) {
	url := fmt.Sprintf("%s/api/v1/storage/get_asset_count", s.url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	s.setCredential(req)

	rsp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	if rsp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(rsp.Body)
		return nil, fmt.Errorf("status code %d %s", rsp.StatusCode, string(buf))
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	ret := &Result{}
	err = json.Unmarshal(body, ret)
	if err != nil {
		return nil, err
	}

	if ret.Code != 0 {
		return nil, fmt.Errorf(fmt.Sprintf("code: %d, err: %d, msg: %s", ret.Code, ret.Err, ret.Msg))
	}

	assetCount := &AssetCountInfo{}
	err = interfaceToStruct(ret.Data, assetCount)
	if err != nil {
		return nil, err
	}

	return assetCount, nil
}

func (s *webserver) setCredential(r *http.Request) {
	if s.apiKey != "" {
		r.Header.Set("apikey", s.apiKey)
	}
	if s.token != "" {
		r.Header.Set("jwtauthorization", fmt.Sprintf("Bearer %s", s.token))
	}
}

func interfaceToStruct(input interface{}, output interface{}) error {
	buf, err := json.Marshal(input)
	if err != nil {
		return err
	}
	err = json.Unmarshal(buf, output)
	if err != nil {
		return err
	}
	return nil
}

// CIDToHash converts a CID string to its corresponding hash string.
func CIDToHash(cidString string) (string, error) {
	cid, err := cid.Decode(cidString)
	if err != nil {
		return "", err
	}

	return cid.Hash().String(), nil
}
