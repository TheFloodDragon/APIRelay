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
	q.StartTime, _ = strconv.ParseInt(c.DefaultQuery("start_time", "0"), 10, 64)
	q.EndTime, _ = strconv.ParseInt(c.DefaultQuery("end_time", "0"), 10, 64)

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
