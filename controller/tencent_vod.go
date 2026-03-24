package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type tencentVODCallbackPayload struct {
	EventType                 string `json:"EventType"`
	ProcedureStateChangeEvent struct {
		TaskId   string `json:"TaskId"`
		Status   string `json:"Status"`
		ErrMsg   string `json:"ErrMsg"`
		Message  string `json:"Message"`
		FileUrl  string `json:"FileUrl"`
		Progress int    `json:"Progress"`
	} `json:"ProcedureStateChangeEvent"`
}

func TencentVODCallback(c *gin.Context) {
	var payload tencentVODCallbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	upstreamTaskID := strings.TrimSpace(payload.ProcedureStateChangeEvent.TaskId)
	if upstreamTaskID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	task, exist, err := model.GetByPlatformAndUpstreamTaskID(constant.TaskPlatform("58"), upstreamTaskID)
	if err != nil || !exist || task == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	channel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil || channel == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	settings := channel.GetOtherSettings()
	if !settings.TencentVODCallbackEnabled || settings.TencentVODCallbackSecret == "" || c.Query("token") != settings.TencentVODCallbackSecret {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	taskInfo := &relaycommon.TaskInfo{
		TaskID:   upstreamTaskID,
		Progress: progressFromTencentCallback(payload.ProcedureStateChangeEvent.Progress),
		Url:      strings.TrimSpace(payload.ProcedureStateChangeEvent.FileUrl),
		Reason:   firstNonEmpty(payload.ProcedureStateChangeEvent.ErrMsg, payload.ProcedureStateChangeEvent.Message),
	}
	switch strings.ToUpper(strings.TrimSpace(payload.ProcedureStateChangeEvent.Status)) {
	case "WAITING":
		taskInfo.Status = string(model.TaskStatusQueued)
	case "PROCESSING":
		taskInfo.Status = string(model.TaskStatusInProgress)
	case "FINISH":
		if taskInfo.Url != "" {
			taskInfo.Status = string(model.TaskStatusSuccess)
		} else {
			taskInfo.Status = string(model.TaskStatusFailure)
		}
	case "ABORTED":
		taskInfo.Status = string(model.TaskStatusFailure)
	default:
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	decision, err := service.ApplyTencentVODTaskUpdate(c.Request.Context(), task, taskInfo, time.Now().Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if decision.ShouldRefund {
		service.RefundTaskQuota(c.Request.Context(), task, task.FailReason)
	}
	if decision.ShouldSettle {
		if service.GetTaskAdaptorFunc != nil {
			if adaptor := service.GetTaskAdaptorFunc(task.Platform); adaptor != nil {
				info := &relaycommon.RelayInfo{}
				info.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: channel.GetBaseURL()}
				info.ApiKey = channel.Key
				adaptor.Init(info)
				service.SettleTaskBillingOnComplete(c.Request.Context(), adaptor, task, taskInfo)
			}
		}
	}
	common.ApiSuccess(c, gin.H{"updated": decision.Updated})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func progressFromTencentCallback(progress int) string {
	if progress <= 0 {
		return ""
	}
	if progress >= 100 {
		return "100%"
	}
	return strconv.Itoa(progress) + "%"
}
