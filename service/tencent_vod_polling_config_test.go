package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestShouldPollTencentVODChannel(t *testing.T) {
	t.Run("non tencent vod channel always uses existing polling flow", func(t *testing.T) {
		channel := &model.Channel{Type: constant.ChannelTypeOpenAI}
		assert.True(t, shouldPollTencentVODChannel(channel))
	})

	t.Run("tencent vod polling disabled by fallback switch", func(t *testing.T) {
		channel := &model.Channel{Type: constant.ChannelTypeTencentVOD}
		channel.SetOtherSettings(dto.ChannelOtherSettings{
			TencentVODAutoQueryEnabled:       true,
			TencentVODPollingFallbackEnabled: false,
		})
		assert.False(t, shouldPollTencentVODChannel(channel))
	})

	t.Run("tencent vod polling disabled by auto query switch", func(t *testing.T) {
		channel := &model.Channel{Type: constant.ChannelTypeTencentVOD}
		channel.SetOtherSettings(dto.ChannelOtherSettings{
			TencentVODAutoQueryEnabled:       false,
			TencentVODPollingFallbackEnabled: true,
		})
		assert.False(t, shouldPollTencentVODChannel(channel))
	})

	t.Run("tencent vod polling enabled when both switches are on", func(t *testing.T) {
		channel := &model.Channel{Type: constant.ChannelTypeTencentVOD}
		channel.SetOtherSettings(dto.ChannelOtherSettings{
			TencentVODAutoQueryEnabled:       true,
			TencentVODPollingFallbackEnabled: true,
		})
		assert.True(t, shouldPollTencentVODChannel(channel))
	})
}
