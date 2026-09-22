package keypool

// ResetForTest 重置全局管理器为一个使用默认配置的全新实例。
//
// 仅供测试使用：清除所有 per-key 内存状态与策略配置，避免用例间串扰。
// 不重置 initOnce（sync.Once 不可重置），而是直接替换 globalManager 指针——
// 后续 GetManager 会看到这个新实例（Load 非 nil 时不会再走 InitManager）。
func ResetForTest() {
	globalManager.Store(&Manager{cfg: DefaultConfig()})
}

// ResetForTestWithConfig 与 ResetForTest 相同，但使用指定配置。
func ResetForTestWithConfig(cfg Config) {
	globalManager.Store(&Manager{cfg: cfg.normalized()})
}
