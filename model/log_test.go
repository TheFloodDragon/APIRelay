package model

import (
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
)

func setupLogTestDB(t *testing.T) {
	t.Helper()
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	DB.Exec("DELETE FROM logs")
	DB.Exec("DELETE FROM settings")
	invalidateModelHealthConfigCache()
}

func seedLog(t *testing.T, l *Log) *Log {
	t.Helper()
	if err := CreateLog(l); err != nil {
		t.Fatalf("create log: %v", err)
	}
	return l
}

func TestListLogsFilters(t *testing.T) {
	setupLogTestDB(t)
	base := int64(1_700_000_000_000)
	seedLog(t, &Log{RequestId: "req-1", UpstreamRequestId: "up-openai-1", Type: LogTypeConsume, TokenName: "tok-a", ChannelId: 1, SrcModel: "gpt-4o", Status: 200, CreatedAt: base + 100})
	seedLog(t, &Log{RequestId: "req-2", UpstreamRequestId: "up-claude-2", Type: LogTypeError, TokenName: "tok-b", ChannelId: 2, SrcModel: "claude-3", Status: 429, CreatedAt: base + 200})
	seedLog(t, &Log{RequestId: "req-3", UpstreamRequestId: "up-openai-3", Type: LogTypeError, TokenName: "tok-a", ChannelId: 1, SrcModel: "gpt-4o", Status: 504, CreatedAt: base + 300})

	cases := []struct {
		name string
		q    LogQuery
		want []string
	}{
		{name: "type", q: LogQuery{Type: LogTypeError}, want: []string{"req-3", "req-2"}},
		{name: "status", q: LogQuery{Status: 429}, want: []string{"req-2"}},
		{name: "token", q: LogQuery{TokenName: "tok-a"}, want: []string{"req-3", "req-1"}},
		{name: "channel", q: LogQuery{ChannelId: 2}, want: []string{"req-2"}},
		{name: "model", q: LogQuery{Model: "gpt-4o"}, want: []string{"req-3", "req-1"}},
		{name: "request id partial", q: LogQuery{RequestId: "req-3"}, want: []string{"req-3"}},
		{name: "upstream request id partial", q: LogQuery{UpstreamRequestId: "openai"}, want: []string{"req-3", "req-1"}},
		{name: "time range", q: LogQuery{StartTime: base + 150, EndTime: base + 250}, want: []string{"req-2"}},
		{name: "combined", q: LogQuery{Type: LogTypeError, TokenName: "tok-a", ChannelId: 1, Model: "gpt-4o", Status: 504}, want: []string{"req-3"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.q
			q.Page = 1
			q.PageSize = 20
			logs, total, err := ListLogs(&q)
			if err != nil {
				t.Fatalf("list logs: %v", err)
			}
			if total != int64(len(tc.want)) {
				t.Fatalf("total = %d, want %d", total, len(tc.want))
			}
			if len(logs) != len(tc.want) {
				t.Fatalf("items = %d, want %d", len(logs), len(tc.want))
			}
			for i, want := range tc.want {
				if logs[i].RequestId != want {
					t.Fatalf("item %d request_id = %q, want %q", i, logs[i].RequestId, want)
				}
			}
		})
	}
}

func TestListModelHealthAggregatesByChannelAndModel(t *testing.T) {
	setupLogTestDB(t)
	base := time.Now().Add(-time.Minute).UnixMilli()
	seedLog(t, &Log{RequestId: "ok-1", Type: LogTypeConsume, ChannelId: 1, SrcModel: "gpt-4o", Status: 200, CreatedAt: base + 100})
	seedLog(t, &Log{RequestId: "ok-2", Type: LogTypeConsume, ChannelId: 1, SrcModel: "gpt-4o", Status: 302, CreatedAt: base + 200})
	seedLog(t, &Log{RequestId: "bad-1", Type: LogTypeConsume, ChannelId: 1, SrcModel: "gpt-4o", Status: 429, Error: "rate limited", CreatedAt: base + 300})
	seedLog(t, &Log{RequestId: "bad-2", Type: LogTypeError, ChannelId: 1, SrcModel: "gpt-4o", Status: 0, Error: "dial timeout", CreatedAt: base + 400})
	seedLog(t, &Log{RequestId: "ok-3", Type: LogTypeConsume, ChannelId: 2, SrcModel: "gpt-4o", Status: 200, CreatedAt: base + 500})
	seedLog(t, &Log{RequestId: "ignored-manage", Type: LogTypeManage, ChannelId: 1, SrcModel: "gpt-4o", Status: 500, Error: "admin failure", CreatedAt: base + 600})
	seedLog(t, &Log{RequestId: "ignored-empty-model", Type: LogTypeConsume, ChannelId: 1, Status: 200, CreatedAt: base + 700})

	byChannel, err := ListModelHealthByChannel()
	if err != nil {
		t.Fatalf("list channel health: %v", err)
	}
	ch1 := byChannel[1]["gpt-4o"]
	if ch1 == nil {
		t.Fatal("missing channel 1 gpt-4o health")
	}
	if ch1.Total != 4 || ch1.Success != 2 || ch1.Failed != 2 {
		t.Fatalf("channel health = total %d success %d failed %d, want 4/2/2", ch1.Total, ch1.Success, ch1.Failed)
	}
	if ch1.Availability != 50 {
		t.Fatalf("availability = %v, want 50", ch1.Availability)
	}
	if ch1.LastUsedAt != base+400 || ch1.LastSuccessAt != base+200 || ch1.LastFailureAt != base+400 {
		t.Fatalf("timestamps = %+v", ch1)
	}
	if ch1.LastError != "dial timeout" {
		t.Fatalf("last_error = %q, want dial timeout", ch1.LastError)
	}
	if ch1.HealthStatus != "unhealthy" || ch1.RecentCount != 100 || ch1.WindowHours != 24 {
		t.Fatalf("policy fields = %+v", ch1)
	}

	byModel, err := ListModelHealthByModel()
	if err != nil {
		t.Fatalf("list model health: %v", err)
	}
	all := byModel["gpt-4o"]
	if all == nil {
		t.Fatal("missing aggregate model health")
	}
	if all.Total != 5 || all.Success != 3 || all.Failed != 2 {
		t.Fatalf("aggregate health = total %d success %d failed %d, want 5/3/2", all.Total, all.Success, all.Failed)
	}
	if all.LastUsedAt != base+500 || all.LastSuccessAt != base+500 || all.LastFailureAt != base+400 {
		t.Fatalf("aggregate timestamps = %+v", all)
	}
}

func TestListModelHealthAppliesTimeAndRecentCountPerAggregationKey(t *testing.T) {
	setupLogTestDB(t)
	if _, err := SaveModelHealthConfig(ModelHealthConfig{RecentCount: 2, WindowHours: 1, HealthyThreshold: 80, WarningThreshold: 50}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	now := time.Now()
	seedLog(t, &Log{RequestId: "outside-window", Type: LogTypeError, ChannelId: 1, SrcModel: "model-a", Status: 500, Error: "old", CreatedAt: now.Add(-2 * time.Hour).UnixMilli()})
	seedLog(t, &Log{RequestId: "ch1-trimmed", Type: LogTypeError, ChannelId: 1, SrcModel: "model-a", Status: 500, Error: "trimmed", CreatedAt: now.Add(-4 * time.Minute).UnixMilli()})
	seedLog(t, &Log{RequestId: "ch1-fail", Type: LogTypeError, ChannelId: 1, SrcModel: "model-a", Status: 503, Error: "latest failure", CreatedAt: now.Add(-3 * time.Minute).UnixMilli()})
	seedLog(t, &Log{RequestId: "ch1-ok", Type: LogTypeConsume, ChannelId: 1, SrcModel: "model-a", Status: 200, CreatedAt: now.Add(-2 * time.Minute).UnixMilli()})
	seedLog(t, &Log{RequestId: "ch2-fail", Type: LogTypeError, ChannelId: 2, SrcModel: "model-a", Status: 429, Error: "limited", CreatedAt: now.Add(-90 * time.Second).UnixMilli()})
	seedLog(t, &Log{RequestId: "ch2-ok", Type: LogTypeConsume, ChannelId: 2, SrcModel: "model-a", Status: 200, CreatedAt: now.Add(-time.Minute).UnixMilli()})

	byChannel, err := ListModelHealthByChannel()
	if err != nil {
		t.Fatalf("list by channel: %v", err)
	}
	for _, channelID := range []int{1, 2} {
		stat := byChannel[channelID]["model-a"]
		if stat == nil || stat.Total != 2 || stat.Success != 1 || stat.Failed != 1 || stat.HealthStatus != "warning" {
			t.Fatalf("channel %d stat = %+v", channelID, stat)
		}
	}
	if got := byChannel[1]["model-a"].LastError; got != "latest failure" {
		t.Fatalf("last error = %q", got)
	}

	byModel, err := ListModelHealthByModel()
	if err != nil {
		t.Fatalf("list by model: %v", err)
	}
	stat := byModel["model-a"]
	if stat == nil || stat.Total != 2 || stat.Success != 1 || stat.Failed != 1 {
		t.Fatalf("model stat = %+v", stat)
	}
	if stat.LastError != "limited" {
		t.Fatalf("model last error = %q, want limited", stat.LastError)
	}
}

func TestEmptyModelHealthStat(t *testing.T) {
	health := EmptyModelHealthStat(7, "never-called")
	if health.ChannelId != 7 || health.Model != "never-called" {
		t.Fatalf("identity = %+v", health)
	}
	if health.Total != 0 || health.Success != 0 || health.Failed != 0 || health.Availability != 0 {
		t.Fatalf("empty health should be zeroed: %+v", health)
	}
}
