package keypool

import (
	"sync"

	"github.com/apirelay/apirelay/model"
)

// Key 健康状态落库队列。
//
// 参照 circuitbreaker.queuePersist：每次状态变更若直接 `go persist(...)`，高失败率时
// 会瞬间派生大量 goroutine 争抢 SQLite 单写连接。改为单 goroutine 顺序消费的有界队列；
// 队列满时丢弃本次快照（状态会在后续变更或进程重启时从内存/DB 重新对齐，丢一帧不影响正确性）。
const persistQueueSize = 256

var (
	persistQueue     chan *model.ChannelKey
	persistQueueOnce sync.Once
)

// queuePersist 将一份 Key 健康快照放入落库队列。虚拟 Key（Id<=0）不落库。
func queuePersist(snapshot *model.ChannelKey) {
	if snapshot == nil || snapshot.Id <= 0 {
		return
	}
	persistQueueOnce.Do(func() {
		persistQueue = make(chan *model.ChannelKey, persistQueueSize)
		go func() {
			for s := range persistQueue {
				persistSnapshot(s)
			}
		}()
	})
	select {
	case persistQueue <- snapshot:
	default:
		// 队列积压：丢弃最新快照，避免阻塞请求路径。
	}
}

// persistSnapshot 将快照写入 channel_keys 运行时字段（版本控制丢弃过期快照）。
func persistSnapshot(snapshot *model.ChannelKey) {
	if snapshot == nil || snapshot.Id <= 0 || model.DB == nil {
		return
	}
	_ = model.UpsertChannelKeyHealth(snapshot)
}
