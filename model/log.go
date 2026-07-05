package model

import "github.com/apirelay/apirelay/common/logger"

// Log 是一条调用日志（核心需求：运行调用日志完善）。
type Log struct {
	Id                int    `json:"id" gorm:"primaryKey"`
	RequestId         string `json:"request_id" gorm:"size:64;index"`
	UpstreamRequestId string `json:"upstream_request_id" gorm:"size:128"`
	CreatedAt         int64  `json:"created_at" gorm:"index"`
	Type              int    `json:"type" gorm:"index"` // 见 LogType*

	UserId    int    `json:"user_id" gorm:"index"`
	TokenId   int    `json:"token_id" gorm:"index"`
	TokenName string `json:"token_name" gorm:"size:128"`

	ChannelId   int    `json:"channel_id" gorm:"index"`
	ChannelName string `json:"channel_name" gorm:"size:128"`
	Group       string `json:"group" gorm:"size:64"`

	// EndpointType 对外协议；ApiType 上游协议名（便于观察协议互转）
	EndpointType string `json:"endpoint_type" gorm:"size:32"`
	ApiType      string `json:"api_type" gorm:"size:32"`

	SrcModel    string `json:"src_model" gorm:"size:128;index"` // 客户端请求的模型
	MappedModel string `json:"mapped_model" gorm:"size:128"`    // 实际转发到上游的模型

	IsStream         bool  `json:"is_stream"`
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	TotalTokens      int   `json:"total_tokens"`
	Quota            int64 `json:"quota"`

	UseTimeMs   int `json:"use_time_ms"`   // 总耗时
	FirstByteMs int `json:"first_byte_ms"` // 首字节延迟（流式）

	Status  int    `json:"status"` // HTTP 状态码
	Ip      string `json:"ip" gorm:"size:64"`
	Error   string `json:"error" gorm:"type:text"`
	Content string `json:"content" gorm:"type:text"`
}

const (
	LogTypeConsume = 1 // 正常消费
	LogTypeError   = 2 // 失败
	LogTypeManage  = 3 // 管理操作
)

// CreateLog 写入一条日志（同步）。
func CreateLog(l *Log) error {
	if l.CreatedAt == 0 {
		l.CreatedAt = nowMilli()
	}
	if err := DB.Create(l).Error; err != nil {
		logger.L().Error("create log failed")
		return err
	}
	return nil
}

// LogQuery 日志筛选条件。
type LogQuery struct {
	UserId    int
	TokenName string
	ChannelId int
	Model     string
	Type      int
	Status    int
	StartTime int64
	EndTime   int64
	Page      int
	PageSize  int
}

// ListLogs 分页查询日志。
func ListLogs(q *LogQuery) ([]*Log, int64, error) {
	tx := DB.Model(&Log{})
	if q.UserId > 0 {
		tx = tx.Where("user_id = ?", q.UserId)
	}
	if q.TokenName != "" {
		tx = tx.Where("token_name = ?", q.TokenName)
	}
	if q.ChannelId > 0 {
		tx = tx.Where("channel_id = ?", q.ChannelId)
	}
	if q.Model != "" {
		tx = tx.Where("src_model = ?", q.Model)
	}
	if q.Type > 0 {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.Status > 0 {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.StartTime > 0 {
		tx = tx.Where("created_at >= ?", q.StartTime)
	}
	if q.EndTime > 0 {
		tx = tx.Where("created_at <= ?", q.EndTime)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 200 {
		q.PageSize = 20
	}
	var logs []*Log
	err := tx.Order("id desc").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&logs).Error
	return logs, total, err
}

// LogStat 仪表盘统计聚合结果。
type LogStat struct {
	TotalRequests   int64 `json:"total_requests"`
	TotalPromptTk   int64 `json:"total_prompt_tokens"`
	TotalCompletion int64 `json:"total_completion_tokens"`
	TotalQuota      int64 `json:"total_quota"`
}

// SumLogStat 统计指定时间范围内的汇总。
func SumLogStat(start, end int64) (*LogStat, error) {
	var s LogStat
	tx := DB.Model(&Log{}).Where("type = ?", LogTypeConsume)
	if start > 0 {
		tx = tx.Where("created_at >= ?", start)
	}
	if end > 0 {
		tx = tx.Where("created_at <= ?", end)
	}
	row := tx.Select(
		"COUNT(*) as total_requests",
		"COALESCE(SUM(prompt_tokens),0) as total_prompt_tk",
		"COALESCE(SUM(completion_tokens),0) as total_completion",
		"COALESCE(SUM(quota),0) as total_quota",
	).Row()
	if err := row.Scan(&s.TotalRequests, &s.TotalPromptTk, &s.TotalCompletion, &s.TotalQuota); err != nil {
		return nil, err
	}
	return &s, nil
}
