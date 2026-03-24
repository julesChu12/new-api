package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTencentVODTaskUpdate_SuccessTerminalTransition(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 101, 101, 101
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "sk-tencent-vod-success", 8000)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, 3000, tokenID, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform("58")
	task.UpstreamTaskID = "upstream_success_1"
	task.Status = model.TaskStatusSubmitted
	require.NoError(t, model.DB.Create(task).Error)

	result := &relaycommon.TaskInfo{
		TaskID:   task.UpstreamTaskID,
		Status:   string(model.TaskStatusSuccess),
		Progress: "100%",
		Url:      "https://example.com/result.png",
	}

	decision, err := ApplyTencentVODTaskUpdate(ctx, task, result, time.Now().Unix())
	require.NoError(t, err)
	assert.True(t, decision.Updated)
	assert.True(t, decision.ShouldSettle)
	assert.False(t, decision.ShouldRefund)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Equal(t, "https://example.com/result.png", task.PrivateData.ResultURL)
}

func TestApplyTencentVODTaskUpdate_DuplicateFailureDoesNotRefundTwice(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 102, 102, 102
	const initQuota, preConsumed = 12000, 4000
	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-tencent-vod-failure", 7000)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Platform = constant.TaskPlatform("58")
	task.UpstreamTaskID = "upstream_failure_1"
	task.Status = model.TaskStatusInProgress
	require.NoError(t, model.DB.Create(task).Error)

	failed := &relaycommon.TaskInfo{
		TaskID:   task.UpstreamTaskID,
		Status:   string(model.TaskStatusFailure),
		Progress: "100%",
		Reason:   "upstream failed",
	}

	firstDecision, err := ApplyTencentVODTaskUpdate(ctx, task, failed, time.Now().Unix())
	require.NoError(t, err)
	assert.True(t, firstDecision.ShouldRefund)

	RefundTaskQuota(ctx, task, failed.Reason)

	reloaded, exist, err := model.GetByTaskId(userID, task.TaskID)
	require.NoError(t, err)
	require.True(t, exist)
	require.NotNil(t, reloaded)

	secondDecision, err := ApplyTencentVODTaskUpdate(ctx, reloaded, failed, time.Now().Unix())
	require.NoError(t, err)
	assert.False(t, secondDecision.ShouldRefund)
	assert.False(t, secondDecision.ShouldSettle)
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, int64(1), countLogs(t))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestApplyTencentVODTaskUpdate_TerminalStateDoesNotRegress(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	task := &model.Task{
		TaskID:         "task_terminal_no_regress",
		Platform:       constant.TaskPlatform("58"),
		UpstreamTaskID: "upstream_terminal_no_regress",
		Status:         model.TaskStatusSuccess,
		Progress:       "100%",
		FinishTime:     time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/final.png",
		},
	}

	decision, err := ApplyTencentVODTaskUpdate(ctx, task, &relaycommon.TaskInfo{
		TaskID:   task.UpstreamTaskID,
		Status:   string(model.TaskStatusFailure),
		Progress: "100%",
		Reason:   "late conflicting failure",
	}, time.Now().Unix())
	require.NoError(t, err)
	assert.False(t, decision.Updated)
	assert.False(t, decision.ShouldRefund)
	assert.False(t, decision.ShouldSettle)
	assert.EqualValues(t, model.TaskStatusSuccess, task.Status)
	assert.Equal(t, "https://example.com/final.png", task.PrivateData.ResultURL)
	assert.NotZero(t, task.FinishTime)
}
