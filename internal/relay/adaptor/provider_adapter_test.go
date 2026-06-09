package adaptor_test

import (
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor/anthropic"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor/gemini"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor/openai"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

func TestProviderAdapterNeedsTransform(t *testing.T) {
	tests := []struct {
		name    string
		adapter adaptor.ProviderAdapter
		format  constant.RelayFormat
		want    bool
	}{
		{name: "openai passthrough", adapter: openai.NewAdaptor(), format: constant.RelayFormatOpenAI, want: false},
		{name: "openai responses passthrough", adapter: openai.NewAdaptor(), format: constant.RelayFormatOpenAIResponses, want: false},
		{name: "openai transforms anthropic", adapter: openai.NewAdaptor(), format: constant.RelayFormatAnthropic, want: true},
		{name: "anthropic passthrough", adapter: anthropic.NewAdaptor(), format: constant.RelayFormatAnthropic, want: false},
		{name: "anthropic transforms openai", adapter: anthropic.NewAdaptor(), format: constant.RelayFormatOpenAI, want: true},
		{name: "gemini passthrough", adapter: gemini.NewAdaptor(), format: constant.RelayFormatGemini, want: false},
		{name: "gemini transforms openai", adapter: gemini.NewAdaptor(), format: constant.RelayFormatOpenAI, want: true},
	}

	channel := &model.Channel{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.adapter.NeedsTransform(channel, tt.format); got != tt.want {
				t.Fatalf("NeedsTransform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAdaptorImplementsProviderAdapter(t *testing.T) {
	for _, apiType := range []constant.APIType{constant.APITypeOpenAI, constant.APITypeAnthropic, constant.APITypeGemini} {
		if _, ok := adaptor.GetAdaptor(apiType).(adaptor.ProviderAdapter); !ok {
			t.Fatalf("GetAdaptor(%s) does not implement ProviderAdapter", apiType)
		}
	}
}
