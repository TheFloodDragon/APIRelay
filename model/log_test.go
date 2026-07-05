package model

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
)

func setupLogTestDB(t *testing.T) {
	t.Helper()
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	DB.Exec("DELETE FROM logs")
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

func TestListLogsPagination(t *testing.T) {
	setupLogTestDB(t)
	base := int64(1_700_000_000_000)
	seedLog(t, &Log{RequestId: "req-1", Type: LogTypeConsume, Status: 200, CreatedAt: base + 100})
	seedLog(t, &Log{RequestId: "req-2", Type: LogTypeConsume, Status: 200, CreatedAt: base + 200})
	seedLog(t, &Log{RequestId: "req-3", Type: LogTypeConsume, Status: 200, CreatedAt: base + 300})

	q := &LogQuery{Page: 2, PageSize: 1}
	logs, total, err := ListLogs(q)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(logs) != 1 || logs[0].RequestId != "req-2" {
		t.Fatalf("page 2 item = %+v, want req-2", logs)
	}
}
