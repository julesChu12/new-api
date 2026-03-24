package constant

import "testing"

func TestTencentVODChannelTypeRegistration(t *testing.T) {
	if ChannelTypeTencentVOD <= ChannelTypeCodex {
		t.Fatalf("expected Tencent VOD channel type to be registered after existing channel types, got %d", ChannelTypeTencentVOD)
	}

	if got := GetChannelTypeName(ChannelTypeTencentVOD); got != "TencentVOD" {
		t.Fatalf("expected channel type name TencentVOD, got %q", got)
	}

	if ChannelTypeTencentVOD >= len(ChannelBaseURLs) {
		t.Fatalf("expected base url slot for Tencent VOD, len=%d type=%d", len(ChannelBaseURLs), ChannelTypeTencentVOD)
	}

	if got := ChannelBaseURLs[ChannelTypeTencentVOD]; got != "https://vod.tencentcloudapi.com" {
		t.Fatalf("expected Tencent VOD base url, got %q", got)
	}
}
