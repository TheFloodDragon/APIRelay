package model

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apirelay/apirelay/common/logger"

	"go.uber.org/zap"
)

// 异步任务：将日志落库与配额结算从请求主路径剥离，避免阻塞转发。
//
// 设计：单一缓冲 channel + 后台 worker。队列满或 worker 未启动时回退同步执行，
// 保证日志与计费不丢失。进程退出前调用 StopAsyncWorker 优雅 flush。

type asyncTask struct {
	log     *Log // 非 nil 时落库
	settle  *settleTask
	payload *payloadTask // 完整日志载荷（落库后保存）
}

type settleTask struct {
	tokenID  int
	reserved int64
	actual   int64
}

type payloadTask struct {
	capture interface{} // 避免循环依赖，实际是 *relaycommon.FullLogCapture
}

var (
	asyncQueue   chan asyncTask
	asyncWG      sync.WaitGroup
	asyncRunning atomic.Bool
	asyncStop    chan struct{}
	asyncStartMu sync.Mutex
)

const asyncQueueSize = 4096

// StartAsyncWorker 启动后台异步 worker（幂等）。
func StartAsyncWorker() {
	// 用互斥锁串行化 start/stop，并在翻转 running 标志前完成队列创建：
	// enqueue 只有在 asyncRunning.Load()==true 时才读 asyncQueue，
	// 而该变量的写入受同一把锁保护并先于 Swap(true) 完成，避免非同步的并发读写。
	asyncStartMu.Lock()
	defer asyncStartMu.Unlock()
	if asyncRunning.Load() {
		return
	}
	queue := make(chan asyncTask, asyncQueueSize)
	stop := make(chan struct{})
	asyncQueue = queue
	asyncStop = stop
	asyncRunning.Store(true)
	asyncWG.Add(1)
	go func() {
		defer asyncWG.Done()
		for {
			select {
			case t := <-queue:
				processAsyncTask(t)
			case <-stop:
				// 排空剩余任务后退出
				for {
					select {
					case t := <-queue:
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
	asyncStartMu.Lock()
	defer asyncStartMu.Unlock()
	if !asyncRunning.Load() {
		return
	}
	asyncRunning.Store(false)
	close(asyncStop)
	asyncWG.Wait()
	logger.L().Info("async worker stopped")
}

func processAsyncTask(t asyncTask) {
	var logID int
	if t.log != nil {
		if err := CreateLog(t.log); err != nil {
			logger.L().Error("async create log failed", zap.Error(err))
		} else {
			logID = t.log.Id
		}
	}
	if t.settle != nil {
		if err := settleWithRetry(t.settle.tokenID, t.settle.reserved, t.settle.actual); err != nil {
			logger.L().Error("async settle quota failed",
				zap.Int("token_id", t.settle.tokenID),
				zap.Int64("reserved", t.settle.reserved),
				zap.Int64("actual", t.settle.actual),
				zap.Error(err),
			)
		}
	}
	// 保存完整日志载荷（必须在 log 创建后）
	if t.payload != nil && logID > 0 && t.payload.capture != nil {
		if err := saveFullLogPayloadSync(logID, t.payload.capture); err != nil {
			logger.L().Error("async save log payload failed", zap.Int("log_id", logID), zap.Error(err))
		}
	}
}

const (
	settleMaxRetries = 3
	settleRetryDelay = 50 * time.Millisecond
)

// settleWithRetry 对 SettleQuota 进行有限次退避重试，仅针对 SQLite BUSY/locked 类瞬时错误。
// 非锁冲突错误（如额度不足）立即返回，不重试。
func settleWithRetry(tokenID int, reserved, actual int64) error {
	return retrySettle(func() error { return SettleQuota(tokenID, reserved, actual) })
}

// retrySettle 对给定结算操作按需退避重试（便于注入测试）。
func retrySettle(fn func() error) error {
	var err error
	for attempt := 0; attempt < settleMaxRetries; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if !isSQLiteBusyErr(err) {
			return err
		}
		time.Sleep(settleRetryDelay)
	}
	return err
}

// isSQLiteBusyErr 判断是否为 SQLite 写锁竞争类瞬时错误（可重试）。
func isSQLiteBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "busy") ||
		strings.Contains(msg, "sqlite_busy")
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

// AsyncLogWithPayload 异步写入日志并保存完整载荷。
func AsyncLogWithPayload(l *Log, capture interface{}) {
	if l == nil {
		return
	}
	enqueue(asyncTask{log: l, payload: &payloadTask{capture: capture}})
}

// AsyncLogAndSettleWithPayload 异步写入日志、结算配额并保存完整载荷。
func AsyncLogAndSettleWithPayload(l *Log, tokenID int, reserved, actual int64, capture interface{}) {
	t := asyncTask{log: l, payload: &payloadTask{capture: capture}}
	if tokenID > 0 && (reserved != 0 || actual != 0) {
		t.settle = &settleTask{tokenID: tokenID, reserved: reserved, actual: actual}
	}
	enqueue(t)
}
