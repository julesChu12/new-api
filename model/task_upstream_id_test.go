package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUpstreamTaskID_PrefersExplicitField(t *testing.T) {
	task := &Task{
		TaskID:         "task_public_123",
		UpstreamTaskID: "upstream_explicit_123",
		PrivateData:    TaskPrivateData{UpstreamTaskID: "upstream_legacy_123"},
	}

	assert.Equal(t, "upstream_explicit_123", task.GetUpstreamTaskID())
}

func TestGetUpstreamTaskID_FallsBackToLegacyPrivateData(t *testing.T) {
	task := &Task{
		TaskID:      "task_public_456",
		PrivateData: TaskPrivateData{UpstreamTaskID: "upstream_legacy_456"},
	}

	assert.Equal(t, "upstream_legacy_456", task.GetUpstreamTaskID())
}

func TestGetByPlatformAndUpstreamTaskID(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID:         "task_public_lookup",
		Platform:       constant.TaskPlatform("58"),
		UpstreamTaskID: "upstream_lookup_123",
		Status:         TaskStatusSubmitted,
	}
	insertTask(t, task)

	found, exist, err := GetByPlatformAndUpstreamTaskID(constant.TaskPlatform("58"), "upstream_lookup_123")
	require.NoError(t, err)
	require.True(t, exist)
	require.NotNil(t, found)
	assert.Equal(t, task.ID, found.ID)
	assert.Equal(t, "upstream_lookup_123", found.UpstreamTaskID)
}

func TestGetByPlatformAndUpstreamTaskID_EmptyUpstreamID(t *testing.T) {
	found, exist, err := GetByPlatformAndUpstreamTaskID(constant.TaskPlatform("58"), "")
	require.NoError(t, err)
	assert.False(t, exist)
	assert.Nil(t, found)
}
