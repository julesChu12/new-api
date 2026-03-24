package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type TaskTransitionDecision struct {
	Updated      bool
	ShouldRefund bool
	ShouldSettle bool
}

func ApplyTencentVODTaskUpdate(ctx context.Context, task *model.Task, taskResult *relaycommon.TaskInfo, now int64) (*TaskTransitionDecision, error) {
	_ = ctx
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}
	if taskResult == nil {
		return nil, fmt.Errorf("task result is nil")
	}
	decision := &TaskTransitionDecision{}
	snap := task.Snapshot()

	if isTerminalStatus(task.Status) {
		return decision, nil
	}

	targetStatus := model.TaskStatus(taskResult.Status)
	task.Status = targetStatus
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	switch targetStatus {
	case model.TaskStatusQueued:
		if task.Progress == "" {
			task.Progress = "0%"
		}
	case model.TaskStatusInProgress:
		if task.StartTime == 0 {
			task.StartTime = now
		}
		if task.Progress == "" {
			task.Progress = "50%"
		}
	case model.TaskStatusSuccess:
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if task.Progress == "" {
			task.Progress = "100%"
		}
		if taskResult.Url != "" {
			task.PrivateData.ResultURL = taskResult.Url
		}
		decision.ShouldSettle = true
	case model.TaskStatusFailure:
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if task.Progress == "" {
			task.Progress = "100%"
		}
		task.FailReason = taskResult.Reason
		if task.Quota != 0 {
			decision.ShouldRefund = true
		}
	default:
		return nil, fmt.Errorf("unsupported task status: %s", taskResult.Status)
	}

	if isTerminalStatus(targetStatus) {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			return nil, err
		}
		if !won {
			decision.ShouldRefund = false
			decision.ShouldSettle = false
			return decision, nil
		}
		decision.Updated = true
		return decision, nil
	}

	if !snap.Equal(task.Snapshot()) {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			return nil, err
		}
		decision.Updated = won
	}
	return decision, nil
}

func isTerminalStatus(status model.TaskStatus) bool {
	return status == model.TaskStatusSuccess || status == model.TaskStatusFailure
}

func SettleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) {
	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
}

func MarkTaskTimedOut(task *model.Task, now int64, reason string) {
	if task == nil || isTerminalStatus(task.Status) {
		return
	}
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = now
	task.FailReason = reason
	if task.StartTime == 0 {
		task.StartTime = time.Now().Unix()
	}
}
