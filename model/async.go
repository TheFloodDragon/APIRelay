package model

import (
	"sync"
	"sync/atomic"

	"github.com/apirelay/apirelay/common/logger"

	"go.uber.org/zap"
)

// 异步任务：将日志落库与配额结算从请求主路径剥离，避免阻塞转发。
//
// 设计：单一缓冲 channel + 后台 worker。队列满或 worker 未启动时回退同步执行，
// 保证日志与计费不丢失。进程退出前调用 StopAsyncWorker 优雅 flush。

type asyncTask struct {
	log    *Log // 非 nil 时落库
	settle *settleTask
}

type settleTask struct {
	tokenID  int
	reserved int64
	actual   int64
}

var (
	asyncQueue   chan asyncTask
	asyncWG      sync.WaitGroup
	asyncRunning atomic.Bool
	asyncStop    chan struct{}
)

const asyncQueueSize = 4096

// StartAsyncWorker 启动后台异步 worker（幂等）。
func StartAsyncWorker() {
	if asyncRunning.Swap(true) {
		return
	}
	asyncQueue = make(chan asyncTask, asyncQueueSize)
	asyncStop = make(chan struct{})
	asyncWG.Add(1)
	go func() {
		defer asyncWG.Done()
		for {
			select {
			case t := <-asyncQueue:
				processAsyncTask(t)
			case <-asyncStop:
				// 排空剩余任务后退出
				for {
					select {
					case t := <-asyncQueue:
						processAsyncTask(t)
					default:
						return
					}
				}
			}
		}
	}()
	logger.L().Info("async worker started")
}

// StopAsyncWorker 停止 worker 并 flush 剩余任务。
func StopAsyncWorker() {
	if !asyncRunning.Swap(false) {
		return
	}
	close(asyncStop)
	asyncWG.Wait()
	logger.L().Info("async worker stopped")
}

func processAsyncTask(t asyncTask) {
	if t.log != nil {
		if err := CreateLog(t.log); err != nil {
			logger.L().Error("async create log failed", zap.Error(err))
		}
	}
	if t.settle != nil {
		SettleQuota(t.settle.tokenID, t.settle.reserved, t.settle.actual)
	}
}

// enqueue 尝试入队；队列满或 worker 未启动则同步回退。
func enqueue(t asyncTask) {
	if !asyncRunning.Load() || asyncQueue == nil {
		processAsyncTask(t)
		return
	}
	select {
	case asyncQueue <- t:
	default:
		// 队列满：同步回退，保证不丢
		processAsyncTask(t)
	}
}

// AsyncLog 异步写入一条日志。
func AsyncLog(l *Log) {
	if l == nil {
		return
	}
	enqueue(asyncTask{log: l})
}

// AsyncSettle 异步结算配额（reserved -> actual）。
func AsyncSettle(tokenID int, reserved, actual int64) {
	if tokenID <= 0 || (reserved == 0 && actual == 0) {
		return
	}
	enqueue(asyncTask{settle: &settleTask{tokenID: tokenID, reserved: reserved, actual: actual}})
}

// AsyncLogAndSettle 在一个任务里同时落库日志与结算配额。
func AsyncLogAndSettle(l *Log, tokenID int, reserved, actual int64) {
	t := asyncTask{log: l}
	if tokenID > 0 && (reserved != 0 || actual != 0) {
		t.settle = &settleTask{tokenID: tokenID, reserved: reserved, actual: actual}
	}
	enqueue(t)
}
