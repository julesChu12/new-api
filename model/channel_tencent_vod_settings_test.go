package model

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestChannelOtherSettingsTencentVODRoundTrip(t *testing.T) {
	channel := &Channel{}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		TencentVODRegion:                 "ap-guangzhou",
		TencentVODSubAppID:               123456,
		TencentVODDefaultModel:           "hunyuan-image",
		TencentVODDefaultResolution:      "1024x1024",
		TencentVODPollingIntervalSeconds: 30,
		TencentVODPollingTimeoutSeconds:  7200,
		TencentVODAutoQueryEnabled:       true,
		TencentVODSessionIDStrategy:      "task_id",
		TencentVODCallbackEnabled:        true,
		TencentVODPollingFallbackEnabled: true,
		TencentVODCallbackSecret:         "callback-secret",
	})

	settings := channel.GetOtherSettings()

	if settings.TencentVODRegion != "ap-guangzhou" {
		t.Fatalf("expected region to round trip, got %q", settings.TencentVODRegion)
	}
	if settings.TencentVODSubAppID != 123456 {
		t.Fatalf("expected sub app id to round trip, got %d", settings.TencentVODSubAppID)
	}
	if settings.TencentVODDefaultModel != "hunyuan-image" {
		t.Fatalf("expected default model to round trip, got %q", settings.TencentVODDefaultModel)
	}
	if settings.TencentVODDefaultResolution != "1024x1024" {
		t.Fatalf("expected default resolution to round trip, got %q", settings.TencentVODDefaultResolution)
	}
	if settings.TencentVODPollingIntervalSeconds != 30 {
		t.Fatalf("expected polling interval to round trip, got %d", settings.TencentVODPollingIntervalSeconds)
	}
	if settings.TencentVODPollingTimeoutSeconds != 7200 {
		t.Fatalf("expected polling timeout to round trip, got %d", settings.TencentVODPollingTimeoutSeconds)
	}
	if !settings.TencentVODAutoQueryEnabled {
		t.Fatalf("expected auto query enabled to round trip")
	}
	if settings.TencentVODSessionIDStrategy != "task_id" {
		t.Fatalf("expected session id strategy to round trip, got %q", settings.TencentVODSessionIDStrategy)
	}
	if !settings.TencentVODCallbackEnabled {
		t.Fatalf("expected callback enabled to round trip")
	}
	if !settings.TencentVODPollingFallbackEnabled {
		t.Fatalf("expected polling fallback enabled to round trip")
	}
	if settings.TencentVODCallbackSecret != "callback-secret" {
		t.Fatalf("expected callback secret to round trip, got %q", settings.TencentVODCallbackSecret)
	}
}
