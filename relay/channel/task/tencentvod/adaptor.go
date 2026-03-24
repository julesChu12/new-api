package tencentvod

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	channelName          = "tencent_vod"
	createAction         = "CreateAigcImageTask"
	describeAction       = "DescribeTaskDetail"
	vodVersion           = "2018-07-17"
	vodService           = "vod"
	defaultRegion        = "ap-guangzhou"
	defaultResolution    = "1024x1024"
	defaultModel         = "hunyuan-image"
	defaultSessionPrefix = "vod_"
	deferredResponseKey  = "relay_deferred_task_submit_response"
)

var modelList = []string{defaultModel}

type createTaskRequest struct {
	Prompt     string   `json:"Prompt,omitempty"`
	ImageUrl   string   `json:"ImageUrl,omitempty"`
	ImageUrls  []string `json:"ImageUrls,omitempty"`
	Model      string   `json:"Model,omitempty"`
	Resolution string   `json:"Resolution,omitempty"`
	SubAppId   int64    `json:"SubAppId,omitempty"`
	SessionId  string   `json:"SessionId,omitempty"`
}

type describeTaskRequest struct {
	TaskId   string `json:"TaskId,omitempty"`
	SubAppId int64  `json:"SubAppId,omitempty"`
}

type taskImageResult struct {
	Url    string `json:"Url,omitempty"`
	FileId string `json:"FileId,omitempty"`
}

type taskOutput struct {
	ImageResultSet []taskImageResult `json:"ImageResultSet,omitempty"`
}

type taskDetail struct {
	TaskId     string     `json:"TaskId,omitempty"`
	Status     string     `json:"Status,omitempty"`
	ErrCodeExt string     `json:"ErrCodeExt,omitempty"`
	ErrMsg     string     `json:"ErrMsg,omitempty"`
	Output     taskOutput `json:"Output,omitempty"`
}

type vodResponse struct {
	Response struct {
		TaskId    string       `json:"TaskId,omitempty"`
		RequestId string       `json:"RequestId,omitempty"`
		Task      taskDetail   `json:"Task,omitempty"`
		TaskInfo  taskDetail   `json:"TaskInfo,omitempty"`
		TaskSet   []taskDetail `json:"TaskSet,omitempty"`
		ErrCode   string       `json:"ErrCode,omitempty"`
		ErrMsg    string       `json:"ErrMsg,omitempty"`
		Error     struct {
			Code    string `json:"Code,omitempty"`
			Message string `json:"Message,omitempty"`
		} `json:"Error,omitempty"`
	} `json:"Response"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey       string
	baseURL      string
	region       string
	action       string
	timestamp    int64
	payloadHash  string
	channelModel string
	subAppID     int64
	sessionID    string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.region = strings.TrimSpace(info.ChannelOtherSettings.TencentVODRegion)
	if a.region == "" {
		a.region = defaultRegion
	}
	a.action = createAction
	a.timestamp = time.Now().Unix()
	a.channelModel = strings.TrimSpace(info.ChannelOtherSettings.TencentVODDefaultModel)
	if a.channelModel == "" {
		a.channelModel = defaultModel
	}
	a.subAppID = info.ChannelOtherSettings.TencentVODSubAppID
	publicTaskID := ""
	if info.TaskRelayInfo != nil {
		publicTaskID = info.PublicTaskID
	}
	a.sessionID = defaultSessionPrefix + publicTaskID
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeTencentVOD]
	}
	return baseURL, nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	secretID, secretKey, err := parseKey(info.ApiKey)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", buildAuthorization(a.action, a.region, a.timestamp, a.payloadHash, secretID, secretKey))
	req.Header.Set("X-TC-Action", a.action)
	req.Header.Set("X-TC-Version", vodVersion)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(a.timestamp, 10))
	req.Header.Set("X-TC-Region", a.region)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req := v.(relaycommon.TaskSubmitReq)
	resolution := strings.TrimSpace(info.ChannelOtherSettings.TencentVODDefaultResolution)
	if resolution == "" {
		resolution = defaultResolution
	}
	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = a.channelModel
	}
	payload := createTaskRequest{
		Prompt:     req.Prompt,
		Model:      modelName,
		Resolution: resolution,
		SubAppId:   a.subAppID,
		SessionId:  a.sessionID,
	}
	if len(req.Images) > 0 {
		payload.ImageUrls = req.Images
		payload.ImageUrl = req.Images[0]
	} else if strings.TrimSpace(req.Image) != "" {
		payload.ImageUrl = strings.TrimSpace(req.Image)
		payload.ImageUrls = []string{payload.ImageUrl}
	}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &payload); err != nil {
		return nil, err
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	a.payloadHash = sha256Hex(data)
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	var result vodResponse
	if err = common.Unmarshal(responseBody, &result); err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}
	if result.Response.Error.Code != "" {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("%s", result.Response.Error.Message), result.Response.Error.Code, http.StatusBadRequest)
		return
	}
	upstreamTaskID := strings.TrimSpace(result.Response.TaskId)
	if upstreamTaskID == "" {
		upstreamTaskID = strings.TrimSpace(result.Response.Task.TaskId)
	}
	if upstreamTaskID == "" {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("missing upstream task id"), "missing_upstream_task_id", http.StatusInternalServerError)
		return
	}
	publicResponse := dto.TaskResponse[string]{
		Code:    "success",
		Message: "",
		Data:    info.PublicTaskID,
	}
	responseBody, err = common.Marshal(publicResponse)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
		return
	}
	c.Set(deferredResponseKey, struct {
		StatusCode int
		Body       []byte
	}{StatusCode: http.StatusOK, Body: responseBody})
	return upstreamTaskID, responseBody, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return modelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return channelName
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	requestBody := describeTaskRequest{TaskId: taskID}
	if subAppID, ok := body["sub_app_id"].(int64); ok {
		requestBody.SubAppId = subAppID
	} else if subAppID, ok := body["sub_app_id"].(int); ok {
		requestBody.SubAppId = int64(subAppID)
	}
	data, err := common.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	requestURL := strings.TrimRight(baseUrl, "/")
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	secretID, secretKey, err := parseKey(key)
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().Unix()
	payloadHash := sha256Hex(data)
	region := defaultRegion
	if value, ok := body["region"].(string); ok && strings.TrimSpace(value) != "" {
		region = strings.TrimSpace(value)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", buildAuthorization(describeAction, region, timestamp, payloadHash, secretID, secretKey))
	req.Header.Set("X-TC-Action", describeAction)
	req.Header.Set("X-TC-Version", vodVersion)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Region", region)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var result vodResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.Response.Error.Code != "" {
		return nil, fmt.Errorf("%s", result.Response.Error.Message)
	}
	task := result.Response.Task
	if task.TaskId == "" {
		task = result.Response.TaskInfo
	}
	if task.TaskId == "" && len(result.Response.TaskSet) > 0 {
		task = result.Response.TaskSet[0]
	}
	info := &relaycommon.TaskInfo{TaskID: task.TaskId, Reason: task.ErrMsg}
	switch strings.ToUpper(strings.TrimSpace(task.Status)) {
	case "WAITING":
		info.Status = string(model.TaskStatusQueued)
		info.Progress = taskcommon.ProgressQueued
	case "PROCESSING":
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	case "FINISH":
		if len(task.Output.ImageResultSet) > 0 {
			info.Status = string(model.TaskStatusSuccess)
			info.Progress = taskcommon.ProgressComplete
			info.Url = firstResultURL(task.Output.ImageResultSet)
		} else {
			info.Status = string(model.TaskStatusFailure)
			info.Progress = taskcommon.ProgressComplete
			if info.Reason == "" {
				info.Reason = task.ErrCodeExt
			}
		}
	default:
		info.Status = string(model.TaskStatusUnknown)
	}
	return info, nil
}

func parseKey(key string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(key), "|")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tencent vod key")
	}
	secretID := strings.TrimSpace(parts[0])
	secretKey := strings.TrimSpace(parts[1])
	if secretID == "" || secretKey == "" {
		return "", "", fmt.Errorf("invalid tencent vod key")
	}
	return secretID, secretKey, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(data, key string) []byte {
	h := hmac.New(sha256.New, []byte(key))
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func buildAuthorization(action, region string, timestamp int64, payloadHash, secretID, secretKey string) string {
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	canonicalHeaders := fmt.Sprintf("content-type:application/json\nhost:vod.tencentcloudapi.com\nx-tc-action:%s\n", strings.ToLower(action))
	canonicalRequest := fmt.Sprintf("POST\n/\n\n%s\ncontent-type;host;x-tc-action\n%s", canonicalHeaders, payloadHash)
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, vodService)
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s\n%s", timestamp, credentialScope, sha256Hex([]byte(canonicalRequest)))
	secretDate := hmacSHA256(date, "TC3"+secretKey)
	secretService := hmacSHA256(vodService, string(secretDate))
	secretSigning := hmacSHA256("tc3_request", string(secretService))
	signature := hex.EncodeToString(hmacSHA256(stringToSign, string(secretSigning)))
	_ = region
	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host;x-tc-action, Signature=%s", secretID, credentialScope, signature)
}

func firstResultURL(results []taskImageResult) string {
	for _, result := range results {
		if strings.TrimSpace(result.Url) != "" {
			return strings.TrimSpace(result.Url)
		}
		if strings.TrimSpace(result.FileId) != "" {
			return strings.TrimSpace(result.FileId)
		}
	}
	return ""
}
