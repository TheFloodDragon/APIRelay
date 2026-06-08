package controller

import (
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

func TestResponsesAttemptOrderConfiguredModes(t *testing.T) {
	candidate := relayCandidate{Channel: model.Channel{Type: "openai", Config: model.JSONMap{"responses_mode": "native"}}}
	got := responsesAttemptOrder(constant.RelayAppOpenAI, candidate)
	assertAttemptOrder(t, got, []responsesAttemptKind{responsesAttemptNative})

	candidate.Channel.Config = model.JSONMap{"responses_mode": "chat_bridge"}
	got = responsesAttemptOrder(constant.RelayAppCodex, candidate)
	assertAttemptOrder(t, got, []responsesAttemptKind{responsesAttemptChatBridge})
}

func TestResponsesAttemptOrderAutoMode(t *testing.T) {
	openAI := relayCandidate{Channel: model.Channel{Type: "openai"}}
	got := responsesAttemptOrder(constant.RelayAppCodex, openAI)
	assertAttemptOrder(t, got, []responsesAttemptKind{responsesAttemptNative, responsesAttemptChatBridge})

	openAI.Channel.Config = model.JSONMap{"supports_responses": true}
	got = responsesAttemptOrder(constant.RelayAppOpenAI, openAI)
	assertAttemptOrder(t, got, []responsesAttemptKind{responsesAttemptNative, responsesAttemptChatBridge})

	anthropic := relayCandidate{Channel: model.Channel{Type: "anthropic"}}
	got = responsesAttemptOrder(constant.RelayAppCodex, anthropic)
	assertAttemptOrder(t, got, []responsesAttemptKind{responsesAttemptChatBridge})
}

func assertAttemptOrder(t *testing.T, got, want []responsesAttemptKind) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("attempt order length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempt order[%d] = %s, want %s: %#v", i, got[i], want[i], got)
		}
	}
}
