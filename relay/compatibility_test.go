package relay

import (
	"testing"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
)

func TestFilterCompatibleCandidatesKeepsPortableRoute(t *testing.T) {
	ir := &dto.UnifiedRequest{Model: "display", ResponseFormat: &dto.UnifiedResponseFormat{Type: "json_schema"}}
	anthropic := &model.Channel{Id: 1, Name: "anthropic", Type: constant.ChannelTypeAnthropic}
	openai := &model.Channel{Id: 2, Name: "openai", Type: constant.ChannelTypeOpenAI}
	compatible, reasons := filterCompatibleCandidates(constant.EndpointOpenAI, ir, []model.ChannelCandidate{{Channel: anthropic}, {Channel: openai}})
	if len(compatible) != 1 || compatible[0].Channel.Id != openai.Id {
		t.Fatalf("compatible candidates = %+v", compatible)
	}
	if len(reasons) != 1 {
		t.Fatalf("incompatibility reasons = %+v", reasons)
	}
}

func TestFilterCompatibleCandidatesAllRejected(t *testing.T) {
	topK := 10
	ir := &dto.UnifiedRequest{Model: "display", TopK: &topK}
	responses := &model.Channel{Id: 1, Name: "responses", Type: constant.ChannelTypeResponses}
	compatible, reasons := filterCompatibleCandidates(constant.EndpointAnthropic, ir, []model.ChannelCandidate{{Channel: responses}})
	if len(compatible) != 0 || len(reasons) != 1 {
		t.Fatalf("compatible=%+v reasons=%+v", compatible, reasons)
	}
}

func TestNormalizeUsageOnlyEstimatesWhenMissing(t *testing.T) {
	ir := &dto.UnifiedRequest{Messages: []dto.UnifiedMessage{{Role: dto.RoleUser, Content: "hello"}}}
	real := normalizeUsage(ir, &dto.Usage{PromptTokens: 9, CompletionTokens: 2, TotalTokens: 11}, 100)
	if real.Estimated || real.CompletionTokens != 2 {
		t.Fatalf("real usage was overwritten: %+v", real)
	}
	estimated := normalizeUsage(ir, nil, 5)
	if !estimated.Estimated || estimated.CompletionTokens != 2 || estimated.PromptTokens == 0 {
		t.Fatalf("missing usage was not estimated: %+v", estimated)
	}
}
