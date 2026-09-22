package keypool

import (
	"context"
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
)

// setupRecoveryDB 建一个内存库并清空相关表。
func setupRecoveryDB(t *testing.T) {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	model.DB.Exec("DELETE FROM channel_keys")
	model.DB.Exec("DELETE FROM channels")
	resetBackoffForTest()
	ResetForTest()
}

// resetBackoffForTest 清空退避状态，隔离用例。
func resetBackoffForTest() {
	backoffMu.Lock()
	backoffState = map[int]*keyBackoff{}
	backoffMu.Unlock()
}

func TestRecoveryScan_MarksRecoveredOnProbeSuccess(t *testing.T) {
	setupRecoveryDB(t)

	ch := &model.Channel{Name: "rec", Status: model.ChannelStatusEnabled, BaseURL: "https://x.test"}
	if err := model.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	// 一个已失效的 Key。
	k := &model.ChannelKey{
		ChannelId: ch.Id, KeyIndex: 0, Key: "sk", Status: model.ChannelKeyStatusEnabled,
		Available: false, DisabledReason: string(model.DisableReasonAuthFailed),
	}
	if err := model.DB.Create(k).Error; err != nil {
		t.Fatal(err)
	}

	// 探测总是成功。
	probe := func(ctx context.Context, ch *model.Channel, key *model.ChannelKey) error {
		return nil
	}
	runRecoveryScan(context.Background(), RecoveryConfig{IntervalSeconds: 60, MaxBackoffSeconds: 900}, probe)

	got, err := model.GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available {
		t.Fatalf("key should be recovered (Available=true), got %+v", got)
	}
	if got.DisabledReason != "" {
		t.Errorf("disabled reason should be cleared, got %q", got.DisabledReason)
	}
}

func TestRecoveryScan_BackoffOnProbeFailure(t *testing.T) {
	setupRecoveryDB(t)

	ch := &model.Channel{Name: "rec", Status: model.ChannelStatusEnabled, BaseURL: "https://x.test"}
	if err := model.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	k := &model.ChannelKey{
		ChannelId: ch.Id, KeyIndex: 0, Key: "sk", Status: model.ChannelKeyStatusEnabled,
		Available: false, DisabledReason: string(model.DisableReasonUpstreamError),
	}
	if err := model.DB.Create(k).Error; err != nil {
		t.Fatal(err)
	}

	var probeCalls int
	probe := func(ctx context.Context, ch *model.Channel, key *model.ChannelKey) error {
		probeCalls++
		return context.DeadlineExceeded // 总是失败
	}
	cfg := RecoveryConfig{IntervalSeconds: 60, MaxBackoffSeconds: 900}

	// 第一轮：探测一次，失败后进入退避。
	runRecoveryScan(context.Background(), cfg, probe)
	if probeCalls != 1 {
		t.Fatalf("probe calls = %d, want 1", probeCalls)
	}
	// 第二轮（紧接着）：仍在退避窗口内，不应再次探测。
	runRecoveryScan(context.Background(), cfg, probe)
	if probeCalls != 1 {
		t.Fatalf("probe calls = %d, want 1 (still in backoff)", probeCalls)
	}

	// Key 仍失效。
	got, err := model.GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Available {
		t.Error("key should remain disabled after failed probe")
	}
}

func TestRecoveryScan_SkipsDisabledChannel(t *testing.T) {
	setupRecoveryDB(t)

	// 渠道被管理员禁用：即使 Key 失效也不探测。
	ch := &model.Channel{Name: "rec", Status: model.ChannelStatusDisabled, BaseURL: "https://x.test"}
	if err := model.DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	k := &model.ChannelKey{
		ChannelId: ch.Id, KeyIndex: 0, Key: "sk", Status: model.ChannelKeyStatusEnabled,
		Available: false, DisabledReason: string(model.DisableReasonAuthFailed),
	}
	if err := model.DB.Create(k).Error; err != nil {
		t.Fatal(err)
	}

	var probeCalls int
	probe := func(ctx context.Context, ch *model.Channel, key *model.ChannelKey) error {
		probeCalls++
		return nil
	}
	runRecoveryScan(context.Background(), RecoveryConfig{IntervalSeconds: 60, MaxBackoffSeconds: 900}, probe)
	if probeCalls != 0 {
		t.Fatalf("probe calls = %d, want 0 (channel disabled)", probeCalls)
	}
}

func TestStartStopRecoveryWorker_Graceful(t *testing.T) {
	setupRecoveryDB(t)

	probe := func(ctx context.Context, ch *model.Channel, key *model.ChannelKey) error { return nil }
	// 启动后立即停止应干净返回（不阻塞、不 panic）。
	StartRecoveryWorker(RecoveryConfig{Enabled: true, IntervalSeconds: 1, MaxBackoffSeconds: 10}, probe)
	// 幂等：重复启动不新增 worker。
	StartRecoveryWorker(RecoveryConfig{Enabled: true, IntervalSeconds: 1, MaxBackoffSeconds: 10}, probe)

	done := make(chan struct{})
	go func() {
		StopRecoveryWorker()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("StopRecoveryWorker did not return in time")
	}

	// 停止后再次 Stop 应无副作用。
	StopRecoveryWorker()
}

func TestStartRecoveryWorker_DisabledOrNilProbeNoop(t *testing.T) {
	setupRecoveryDB(t)

	// Enabled=false：不启动。
	StartRecoveryWorker(RecoveryConfig{Enabled: false, IntervalSeconds: 1}, func(context.Context, *model.Channel, *model.ChannelKey) error { return nil })
	recoveryMu.Lock()
	c1 := recoveryCancel
	recoveryMu.Unlock()
	if c1 != nil {
		t.Fatal("worker should not start when disabled")
	}

	// probe=nil：不启动。
	StartRecoveryWorker(RecoveryConfig{Enabled: true, IntervalSeconds: 1}, nil)
	recoveryMu.Lock()
	c2 := recoveryCancel
	recoveryMu.Unlock()
	if c2 != nil {
		t.Fatal("worker should not start with nil probe")
	}
}
