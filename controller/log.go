package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

// ListLogs GET /api/logs 调用日志查询。
func ListLogs(c *gin.Context) {
	q := &model.LogQuery{
		TokenName:         c.Query("token_name"),
		Model:             c.Query("model"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
	}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q.ChannelId, _ = strconv.Atoi(c.DefaultQuery("channel_id", "0"))
	q.Type, _ = strconv.Atoi(c.DefaultQuery("type", "0"))
	q.Status, _ = strconv.Atoi(c.DefaultQuery("status", "0"))
	q.StatusMin, _ = strconv.Atoi(c.DefaultQuery("status_min", "0"))
	q.StatusMax, _ = strconv.Atoi(c.DefaultQuery("status_max", "0"))
	q.StartTime, _ = strconv.ParseInt(c.DefaultQuery("start_time", "0"), 10, 64)
	q.EndTime, _ = strconv.ParseInt(c.DefaultQuery("end_time", "0"), 10, 64)

	// 可选 bool 筛选
	if c.Query("has_full_record") != "" {
		val := c.Query("has_full_record") == "true"
		q.HasFullRecord = &val
	}
	if c.Query("is_stream") != "" {
		val := c.Query("is_stream") == "true"
		q.IsStream = &val
	}

	logs, total, err := model.ListLogs(q)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"items": logs, "total": total, "page": q.Page, "page_size": q.PageSize})
}

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
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		fail(c, http.StatusBadRequest, "invalid log id")
		return
	}
	var log model.Log
	if err := model.DB.Where("id = ?", id).First(&log).Error; err != nil {
		fail(c, http.StatusNotFound, "log not found")
		return
	}
	resp := gin.H{"log": log}
	if log.HasFullRecord {
		payload, err := model.GetLogPayload(id)
		if err == nil {
			resp["payload"] = payload
		}
	}
	ok(c, resp)
}
