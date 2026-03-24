package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTaskAdaptor_ReturnsTencentVODAdaptor(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("58"))
	require.NotNil(t, adaptor)
	assert.Equal(t, "tencent_vod", adaptor.GetChannelName())
}

func TestTencentVODTaskAdaptor_BuildRequestURL(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("58"))
	require.NotNil(t, adaptor)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://vod.tencentcloudapi.com",
		},
	}
	adaptor.Init(info)

	url, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://vod.tencentcloudapi.com", url)
}
