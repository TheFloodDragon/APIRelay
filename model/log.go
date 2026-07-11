package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apirelay/apirelay/common/logger"
)

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

	// HasFullRecord 标记本条日志是否有关联的完整请求/响应详情。
	HasFullRecord         bool `json:"has_full_record" gorm:"index"`
	PayloadOriginalSize   int  `json:"payload_original_size"`
	PayloadCompressedSize int  `json:"payload_compressed_size"`
}

// LogPayload 存储一条日志的完整请求/响应详情，gzip 压缩 JSON 后存入 blob。
type LogPayload struct {
	LogId            int    `json:"-" gorm:"primaryKey;index"`
	CompressedData   []byte `json:"-" gorm:"type:blob"`
	OriginalSize     int    `json:"original_size"`
	CompressedSize   int    `json:"compressed_size"`
	CompressionAlgo  string `json:"compression_algo" gorm:"size:16"` // "gzip"
	ClientRequest    string `json:"client_request" gorm:"-"`
	UpstreamRequest  string `json:"upstream_request" gorm:"-"`
	UpstreamResponse string `json:"upstream_response" gorm:"-"`
	ClientResponse   string `json:"client_response" gorm:"-"`
	FailoverAttempts string `json:"failover_attempts" gorm:"-"` // 切换渠道失败记录
}

// FullLogData 是 LogPayload 展开的数据结构（未压缩）
type FullLogData struct {
	ClientRequest    string `json:"client_request,omitempty"`
	UpstreamRequest  string `json:"upstream_request,omitempty"`
	UpstreamResponse string `json:"upstream_response,omitempty"`
	ClientResponse   string `json:"client_response,omitempty"`
	FailoverAttempts string `json:"failover_attempts,omitempty"`
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

// CreateLogPayload gzip 压缩并写入完整日志载荷。
func CreateLogPayload(logID int, data *FullLogData) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	originalSize := len(jsonBytes)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(jsonBytes); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	compressed := buf.Bytes()

	payload := &LogPayload{
		LogId:           logID,
		CompressedData:  compressed,
		OriginalSize:    originalSize,
		CompressedSize:  len(compressed),
		CompressionAlgo: "gzip",
	}
	if err := DB.Create(payload).Error; err != nil {
		return err
	}
	return DB.Model(&Log{}).Where("id = ?", logID).Updates(map[string]any{
		"has_full_record":         true,
		"payload_original_size":   originalSize,
		"payload_compressed_size": len(compressed),
	}).Error
}

// GetLogPayload 读取并解压指定日志的完整载荷。
func GetLogPayload(logID int) (*FullLogData, error) {
	var payload LogPayload
	if err := DB.Where("log_id = ?", logID).First(&payload).Error; err != nil {
		return nil, err
	}
	gr, err := gzip.NewReader(bytes.NewReader(payload.CompressedData))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(gr); err != nil {
		return nil, err
	}
	var data FullLogData
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// saveFullLogPayloadSync 同步保存完整日志载荷（内部使用）。
// capture 是 interface{}，实际为 *relaycommon.FullLogCapture，避免循环依赖。
func saveFullLogPayloadSync(logID int, capture interface{}) error {
	if capture == nil {
		return nil
	}

	// 序列化各部分（使用 JSON 作为中间格式避免类型依赖）
	captureJSON, err := json.Marshal(capture)
	if err != nil {
		return err
	}

	// 解析为通用结构
	var genericCapture captureData
	if err := json.Unmarshal(captureJSON, &genericCapture); err != nil {
		return err
	}

	// 序列化各部分为 JSON 字符串
	data := &FullLogData{
		ClientRequest:    serializeClientReq(&genericCapture),
		UpstreamRequest:  serializeUpstreamReq(&genericCapture),
		UpstreamResponse: serializeUpstreamResp(&genericCapture),
		ClientResponse:   serializeClientResp(&genericCapture),
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	originalSize := len(jsonBytes)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(jsonBytes); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	compressed := buf.Bytes()

	payload := &LogPayload{
		LogId:           logID,
		CompressedData:  compressed,
		OriginalSize:    originalSize,
		CompressedSize:  len(compressed),
		CompressionAlgo: "gzip",
	}
	if err := DB.Create(payload).Error; err != nil {
		return err
	}
	return DB.Model(&Log{}).Where("id = ?", logID).Updates(map[string]any{
		"has_full_record":         true,
		"payload_original_size":   originalSize,
		"payload_compressed_size": len(compressed),
	}).Error
}

type captureData struct {
	ClientMethod        string            `json:"client_method"`
	ClientPath          string            `json:"client_path"`
	ClientQuery         string            `json:"client_query"`
	ClientHeaders       map[string]string `json:"client_headers"`
	ClientBody          []byte            `json:"client_body"`
	UpstreamURL         string            `json:"upstream_url"`
	UpstreamHeaders     map[string]string `json:"upstream_headers"`
	UpstreamBody        []byte            `json:"upstream_body"`
	UpstreamStatus      int               `json:"upstream_status"`
	UpstreamRespHeaders map[string]string `json:"upstream_resp_headers"`
	UpstreamRespBody    []byte            `json:"upstream_resp_body"`
	ClientRespStatus    int               `json:"client_resp_status"`
	ClientRespHeaders   map[string]string `json:"client_resp_headers"`
	ClientRespBody      []byte            `json:"client_resp_body"`
}

func serializeClientReq(c *captureData) string {
	if c.ClientHeaders == nil && len(c.ClientBody) == 0 {
		return ""
	}
	data := map[string]any{
		"method":  c.ClientMethod,
		"path":    c.ClientPath,
		"query":   c.ClientQuery,
		"headers": c.ClientHeaders,
		"body":    string(c.ClientBody),
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func serializeUpstreamReq(c *captureData) string {
	if c.UpstreamHeaders == nil && len(c.UpstreamBody) == 0 {
		return ""
	}
	data := map[string]any{
		"url":     c.UpstreamURL,
		"headers": c.UpstreamHeaders,
		"body":    string(c.UpstreamBody),
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func serializeUpstreamResp(c *captureData) string {
	if c.UpstreamRespHeaders == nil && len(c.UpstreamRespBody) == 0 {
		return ""
	}
	data := map[string]any{
		"status":  c.UpstreamStatus,
		"headers": c.UpstreamRespHeaders,
		"body":    string(c.UpstreamRespBody),
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func serializeClientResp(c *captureData) string {
	if c.ClientRespHeaders == nil && len(c.ClientRespBody) == 0 {
		return ""
	}
	data := map[string]any{
		"status":  c.ClientRespStatus,
		"headers": c.ClientRespHeaders,
		"body":    string(c.ClientRespBody),
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// LogQuery 日志筛选条件。
type LogQuery struct {
	UserId            int
	TokenName         string
	ChannelId         int
	Model             string
	RequestId         string
	UpstreamRequestId string
	Type              int
	Status            int
	StatusMin         int   // 状态码最小值（包含）
	StatusMax         int   // 状态码最大值（包含）
	HasFullRecord     *bool // nil: 不筛选；true: 仅有详情；false: 仅无详情
	IsStream          *bool // nil: 不筛选；true: 仅流式；false: 仅非流式
	StartTime         int64
	EndTime           int64
	Page              int
	PageSize          int
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
	if q.RequestId != "" {
		tx = tx.Where("request_id LIKE ?", "%"+q.RequestId+"%")
	}
	if q.UpstreamRequestId != "" {
		tx = tx.Where("upstream_request_id LIKE ?", "%"+q.UpstreamRequestId+"%")
	}
	if q.Type > 0 {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.Status > 0 {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.StatusMin > 0 {
		tx = tx.Where("status >= ?", q.StatusMin)
	}
	if q.StatusMax > 0 {
		tx = tx.Where("status <= ?", q.StatusMax)
	}
	if q.HasFullRecord != nil {
		tx = tx.Where("has_full_record = ?", *q.HasFullRecord)
	}
	if q.IsStream != nil {
		tx = tx.Where("is_stream = ?", *q.IsStream)
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

func ListModelLastUsed() (map[string]int64, error) {
	type row struct {
		Model      string
		LastUsedAt int64
	}
	var rows []row
	if err := DB.Model(&Log{}).
		Select("src_model AS model, MAX(created_at) AS last_used_at").
		Where("src_model <> ''").
		Group("src_model").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Model] = item.LastUsedAt
	}
	return result, nil
}

// ModelHealthStat 是基于真实调用日志聚合的模型可用性统计。
type ModelHealthStat struct {
	ChannelId     int     `json:"channel_id,omitempty"`
	Model         string  `json:"model,omitempty"`
	Total         int64   `json:"total"`
	Success       int64   `json:"success"`
	Failed        int64   `json:"failed"`
	Availability  float64 `json:"availability"` // 成功率百分比，0-100。
	LastUsedAt    int64   `json:"last_used_at"`
	LastSuccessAt int64   `json:"last_success_at"`
	LastFailureAt int64   `json:"last_failure_at"`
	LastError     string  `json:"last_error,omitempty"`
}

// EmptyModelHealthStat 返回未调用模型的空健康对象，避免前端将无数据误判为 0% 不可用。
func EmptyModelHealthStat(channelID int, modelName string) *ModelHealthStat {
	return &ModelHealthStat{ChannelId: channelID, Model: modelName}
}

// ListModelHealthByChannel 按 channel_id + src_model 聚合调用健康统计。
func ListModelHealthByChannel() (map[int]map[string]*ModelHealthStat, error) {
	var rows []*ModelHealthStat
	if err := DB.Model(&Log{}).
		Select(modelHealthSelect("channel_id, src_model AS model")).
		Where("src_model <> ? AND type IN ?", "", []int{LogTypeConsume, LogTypeError}).
		Group("channel_id, src_model").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	finalizeModelHealth(rows)
	if err := attachLastFailureErrors(rows, true); err != nil {
		return nil, err
	}
	result := make(map[int]map[string]*ModelHealthStat)
	for _, item := range rows {
		if result[item.ChannelId] == nil {
			result[item.ChannelId] = make(map[string]*ModelHealthStat)
		}
		result[item.ChannelId][item.Model] = item
	}
	return result, nil
}

// ListModelHealthByModel 按 src_model 聚合跨渠道模型总览健康统计。
func ListModelHealthByModel() (map[string]*ModelHealthStat, error) {
	var rows []*ModelHealthStat
	if err := DB.Model(&Log{}).
		Select(modelHealthSelect("src_model AS model")).
		Where("src_model <> ? AND type IN ?", "", []int{LogTypeConsume, LogTypeError}).
		Group("src_model").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	finalizeModelHealth(rows)
	if err := attachLastFailureErrors(rows, false); err != nil {
		return nil, err
	}
	result := make(map[string]*ModelHealthStat, len(rows))
	for _, item := range rows {
		result[item.Model] = item
	}
	return result, nil
}

func modelHealthSelect(prefix string) string {
	success := modelHealthSuccessSQL()
	failure := modelHealthFailureSQL()
	return fmt.Sprintf(`%s,
COUNT(*) AS total,
COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS success,
COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS failed,
MAX(created_at) AS last_used_at,
MAX(CASE WHEN %s THEN created_at ELSE 0 END) AS last_success_at,
MAX(CASE WHEN %s THEN created_at ELSE 0 END) AS last_failure_at`, prefix, success, failure, success, failure)
}

func modelHealthSuccessSQL() string {
	return fmt.Sprintf("type = %d AND status >= 200 AND status < 400 AND COALESCE(error, '') = ''", LogTypeConsume)
}

func modelHealthFailureSQL() string {
	return fmt.Sprintf("type = %d OR status < 200 OR status >= 400 OR COALESCE(error, '') <> ''", LogTypeError)
}

func finalizeModelHealth(rows []*ModelHealthStat) {
	for _, item := range rows {
		if item == nil || item.Total <= 0 {
			continue
		}
		item.Availability = float64(item.Success) / float64(item.Total) * 100
	}
}

func attachLastFailureErrors(rows []*ModelHealthStat, byChannel bool) error {
	if len(rows) == 0 {
		return nil
	}
	byKey := make(map[string]*ModelHealthStat, len(rows))
	for _, item := range rows {
		if item == nil || item.LastFailureAt == 0 {
			continue
		}
		byKey[modelHealthKey(item.ChannelId, item.Model, byChannel)] = item
	}
	if len(byKey) == 0 {
		return nil
	}
	type failureRow struct {
		ChannelId int
		Model     string
		Status    int
		Error     string
		Content   string
	}
	var failures []failureRow
	if err := DB.Model(&Log{}).
		Select("channel_id, src_model AS model, status, error, content").
		Where("src_model <> ? AND type IN ? AND ("+modelHealthFailureSQL()+")", "", []int{LogTypeConsume, LogTypeError}).
		Order("created_at desc, id desc").
		Scan(&failures).Error; err != nil {
		return err
	}
	for _, failure := range failures {
		key := modelHealthKey(failure.ChannelId, failure.Model, byChannel)
		item, ok := byKey[key]
		if !ok || item.LastError != "" {
			continue
		}
		item.LastError = compactLastError(failure.Error, failure.Content, failure.Status)
	}
	return nil
}

func modelHealthKey(channelID int, modelName string, byChannel bool) string {
	if byChannel {
		return fmt.Sprintf("%d\x00%s", channelID, modelName)
	}
	return modelName
}

func compactLastError(errorText, content string, status int) string {
	message := strings.TrimSpace(errorText)
	if message == "" {
		message = strings.TrimSpace(content)
	}
	if message == "" {
		if status > 0 {
			message = fmt.Sprintf("HTTP %d", status)
		} else {
			message = "调用失败"
		}
	}
	runes := []rune(message)
	if len(runes) > 240 {
		message = string(runes[:240]) + "…"
	}
	return message
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
