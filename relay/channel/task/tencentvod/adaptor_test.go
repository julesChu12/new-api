package tencentvod

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchTask_UsesProvidedRegionAndSubAppID(t *testing.T) {
	service.InitHttpClient()
	common.RelayTimeout = 30

	var capturedAction string
	var capturedRegion string
	var requestBody describeTaskRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAction = r.Header.Get("X-TC-Action")
		capturedRegion = r.Header.Get("X-TC-Region")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &requestBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":{"Task":{"TaskId":"upstream_1","Status":"WAITING"}}}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(server.URL, "secret-id|secret-key", map[string]any{
		"task_id":    "upstream_1",
		"region":     "ap-shanghai",
		"sub_app_id": int64(9527),
	}, "")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, describeAction, capturedAction)
	assert.Equal(t, "ap-shanghai", capturedRegion)
	assert.Equal(t, "upstream_1", requestBody.TaskId)
	assert.EqualValues(t, 9527, requestBody.SubAppId)
}
