package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	tencentvod "github.com/QuantumNous/new-api/relay/channel/task/tencentvod"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSubmittedTask_PersistsUpstreamTaskIDMapping(t *testing.T) {
	info := &relaycommon.RelayInfo{
		UserId:     1001,
		UsingGroup: "default",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 58,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_123",
		},
	}
	result := &TaskSubmitResult{
		Platform:       constant.TaskPlatform("58"),
		UpstreamTaskID: "upstream_123",
		TaskData:       []byte(`{"Response":{"TaskId":"upstream_123"}}`),
		Quota:          42,
	}

	task := BuildSubmittedTask(result, info)
	require.NotNil(t, task)
	assert.Equal(t, "task_public_123", task.TaskID)
	assert.Equal(t, "upstream_123", task.UpstreamTaskID)
	assert.Equal(t, "upstream_123", task.PrivateData.UpstreamTaskID)
	assert.Equal(t, 42, task.Quota)
	assert.Equal(t, constant.TaskPlatform("58"), task.Platform)
	assert.Equal(t, "upstream_123", task.GetUpstreamTaskID())
}

func TestTencentVODDoResponse_ReturnsPublicTaskIDToClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	adaptor := &tencentvod.TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_456"},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"Response":{"TaskId":"upstream_456","RequestId":"req_1"}}`)),
	}

	upstreamTaskID, _, taskErr := adaptor.DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "upstream_456", upstreamTaskID)
	assert.Empty(t, recorder.Body.String())

	assert.True(t, WriteDeferredTaskSubmitResponse(c))

	var payload dto.TaskResponse[string]
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "task_public_456", payload.Data)
	assert.NotContains(t, recorder.Body.String(), "upstream_456")
}
