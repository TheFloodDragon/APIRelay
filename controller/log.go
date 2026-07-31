package controller

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const logExportFlushEvery = 500

var logCSVHeader = []string{
	"id",
	"request_id",
	"upstream_request_id",
	"created_at",
	"type",
	"user_id",
	"token_id",
	"token_name",
	"channel_id",
	"channel_name",
	"group",
	"endpoint_type",
	"api_type",
	"src_model",
	"mapped_model",
	"is_stream",
	"prompt_tokens",
	"completion_tokens",
	"total_tokens",
	"cache_creation_input_tokens",
	"cache_read_input_tokens",
	"reasoning_tokens",
	"usage_estimated",
	"quota",
	"use_time_ms",
	"first_byte_ms",
	"status",
	"ip",
	"error",
	"content",
	"has_full_record",
	"payload_original_size",
	"payload_compressed_size",
	"created_at_utc",
}

// logPayloadCSVHeader 是完整导出追加的报文列，仅在 include_payload=true 时写入。
var logPayloadCSVHeader = []string{
	"failover_attempts",
	"client_request",
	"upstream_request",
	"upstream_response",
	"client_response",
}

// ListLogs GET /api/logs 调用日志查询。
func ListLogs(c *gin.Context) {
	q, err := parseLogQuery(c, true)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logs, total, err := model.ListLogs(q)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"items": logs, "total": total, "page": q.Page, "page_size": q.PageSize})
}

// ExportLogs GET /api/logs/export 按当前筛选导出日志 CSV。
// include_payload=true 时追加完整请求/响应报文列。
func ExportLogs(c *gin.Context) {
	q, err := parseLogQuery(c, false)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	includePayload, err := parseBoolQuery(c, "include_payload")
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	withPayload := includePayload != nil && *includePayload
	snapshot, err := model.PrepareLogExport(c.Request.Context(), q)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	scope := "logs"
	if withPayload {
		scope = "logs-full"
	}
	filename := "apirelay-" + scope + "-" + time.Now().UTC().Format("20060102-150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Export-Count", strconv.FormatInt(snapshot.Total, 10))
	c.Status(http.StatusOK)

	buffer := bufio.NewWriterSize(c.Writer, 64*1024)
	if _, err := buffer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		logExportError(c, err)
		return
	}
	writer := csv.NewWriter(buffer)
	header := logCSVHeader
	if withPayload {
		header = append(append([]string{}, logCSVHeader...), logPayloadCSVHeader...)
	}
	if err := writer.Write(header); err != nil {
		logExportError(c, err)
		return
	}

	written := 0
	err = model.WalkLogExportRows(c.Request.Context(), q, snapshot, withPayload, func(item *model.Log, payload *model.FullLogData) error {
		row := logCSVRow(item)
		if withPayload {
			row = append(row, logPayloadCSVRow(payload)...)
		}
		if err := writer.Write(row); err != nil {
			return err
		}
		written++
		if written%logExportFlushEvery == 0 {
			writer.Flush()
			if err := writer.Error(); err != nil {
				return err
			}
			if err := buffer.Flush(); err != nil {
				return err
			}
			c.Writer.Flush()
		}
		return nil
	})
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	if flushErr := buffer.Flush(); err == nil {
		err = flushErr
	}
	if err != nil {
		logExportError(c, err)
	}
}

func logCSVRow(item *model.Log) []string {
	createdAtUTC := ""
	if item.CreatedAt > 0 {
		createdAtUTC = time.UnixMilli(item.CreatedAt).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return []string{
		strconv.Itoa(item.Id),
		item.RequestId,
		item.UpstreamRequestId,
		strconv.FormatInt(item.CreatedAt, 10),
		strconv.Itoa(item.Type),
		strconv.Itoa(item.UserId),
		strconv.Itoa(item.TokenId),
		item.TokenName,
		strconv.Itoa(item.ChannelId),
		item.ChannelName,
		item.Group,
		item.EndpointType,
		item.ApiType,
		item.SrcModel,
		item.MappedModel,
		strconv.FormatBool(item.IsStream),
		strconv.Itoa(item.PromptTokens),
		strconv.Itoa(item.CompletionTokens),
		strconv.Itoa(item.TotalTokens),
		strconv.Itoa(item.CacheCreationInputTokens),
		strconv.Itoa(item.CacheReadInputTokens),
		strconv.Itoa(item.ReasoningTokens),
		strconv.FormatBool(item.UsageEstimated),
		strconv.FormatInt(item.Quota, 10),
		strconv.Itoa(item.UseTimeMs),
		strconv.Itoa(item.FirstByteMs),
		strconv.Itoa(item.Status),
		item.Ip,
		item.Error,
		item.Content,
		strconv.FormatBool(item.HasFullRecord),
		strconv.Itoa(item.PayloadOriginalSize),
		strconv.Itoa(item.PayloadCompressedSize),
		createdAtUTC,
	}
}

func logPayloadCSVRow(payload *model.FullLogData) []string {
	if payload == nil {
		return make([]string, len(logPayloadCSVHeader))
	}
	return []string{
		payload.FailoverAttempts,
		payload.ClientRequest,
		payload.UpstreamRequest,
		payload.UpstreamResponse,
		payload.ClientResponse,
	}
}

func logExportError(c *gin.Context, err error) {
	if c.Request.Context().Err() != nil {
		return
	}
	logger.L().Warn("log export interrupted", zap.Error(err))
}

func parseLogQuery(c *gin.Context, paginate bool) (*model.LogQuery, error) {
	q := &model.LogQuery{
		TokenName:         strings.TrimSpace(c.Query("token_name")),
		Model:             strings.TrimSpace(c.Query("model")),
		RequestId:         strings.TrimSpace(c.Query("request_id")),
		UpstreamRequestId: strings.TrimSpace(c.Query("upstream_request_id")),
	}
	var err error
	if paginate {
		q.Page, err = parseIntQuery(c, "page", 1, 1, 0)
		if err != nil {
			return nil, err
		}
		q.PageSize, err = parseIntQuery(c, "page_size", 20, 1, 200)
		if err != nil {
			return nil, err
		}
	}
	if q.ChannelId, err = parseIntQuery(c, "channel_id", 0, 0, 0); err != nil {
		return nil, err
	}
	if q.Type, err = parseIntQuery(c, "type", 0, 0, 0); err != nil {
		return nil, err
	}
	if q.Status, err = parseIntQuery(c, "status", 0, 0, 0); err != nil {
		return nil, err
	}
	if q.StatusMin, err = parseIntQuery(c, "status_min", 0, 0, 0); err != nil {
		return nil, err
	}
	if q.StatusMax, err = parseIntQuery(c, "status_max", 0, 0, 0); err != nil {
		return nil, err
	}
	if q.StartTime, err = parseInt64Query(c, "start_time"); err != nil {
		return nil, err
	}
	if q.EndTime, err = parseInt64Query(c, "end_time"); err != nil {
		return nil, err
	}
	if q.StatusMin > 0 && q.StatusMax > 0 && q.StatusMin > q.StatusMax {
		return nil, errorsForQuery("status_min must not exceed status_max")
	}
	if q.StartTime > 0 && q.EndTime > 0 && q.StartTime > q.EndTime {
		return nil, errorsForQuery("start_time must not exceed end_time")
	}
	if q.HasFullRecord, err = parseBoolQuery(c, "has_full_record"); err != nil {
		return nil, err
	}
	if q.IsStream, err = parseBoolQuery(c, "is_stream"); err != nil {
		return nil, err
	}
	return q, nil
}

func parseIntQuery(c *gin.Context, name string, defaultValue, minValue, maxValue int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minValue || (maxValue > 0 && value > maxValue) {
		return 0, errorsForQuery(fmt.Sprintf("invalid %s", name))
	}
	return value, nil
}

func parseInt64Query(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, errorsForQuery(fmt.Sprintf("invalid %s", name))
	}
	return value, nil
}

func parseBoolQuery(c *gin.Context, name string) (*bool, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, errorsForQuery(fmt.Sprintf("invalid %s", name))
	}
	return &value, nil
}

func errorsForQuery(message string) error { return fmt.Errorf("invalid log query: %s", message) }

// Dashboard GET /api/dashboard 仪表盘统计。
func Dashboard(c *gin.Context) {
	end := time.Now().UnixMilli()
	start := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	stat, err := model.SumLogStat(start, end)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	channels, _ := model.ListChannels()
	ok(c, gin.H{"stat": stat, "channel_count": len(channels)})
}

// GetLogDetail GET /api/logs/:id 获取单条日志及完整载荷。
func GetLogDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "invalid log id")
		return
	}
	item, err := model.GetLogByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "log not found")
		return
	}
	resp := gin.H{"log": item}
	if item.HasFullRecord {
		payload, err := model.GetLogPayload(id)
		if err == nil {
			resp["payload"] = payload
		}
	}
	ok(c, resp)
}
