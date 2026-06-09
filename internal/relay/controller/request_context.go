package controller

import (
	"io"
	"net/http"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

type RequestContext struct {
	Gin          *gin.Context
	RequestID    string
	StartTime    time.Time
	App          constant.RelayApp
	Mode         constant.RelayMode
	Format       constant.RelayFormat
	Method       string
	OriginalPath string
	Query        string
	// RawBody 保留客户端原始请求体，供透传场景直接转发。
	RawBody []byte
	// Body 是当前控制器路径使用的请求体。普通 relay 中等同 RawBody；Responses bridge 中为 Chat Completions 兼容体。
	Body       []byte
	Meta       relayRequestMeta
	Candidates []relayCandidate
}

func (rc *RelayController) newRequestContext(
	c *gin.Context,
	app constant.RelayApp,
	mode constant.RelayMode,
	format constant.RelayFormat,
) (*RequestContext, bool) {
	startTime := time.Now()
	requestID := requestID(c)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "读取请求失败", "invalid_request_error", err.Error())
		return nil, false
	}

	meta, err := parseRequestMeta(c, body, mode, format)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, "", http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, "请求格式错误", "invalid_request_error", err.Error())
		return nil, false
	}
	if meta.Model == "" {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, "", http.StatusBadRequest, "缺少 model 参数")
		writeRelayError(c, http.StatusBadRequest, "缺少 model 参数", "invalid_request_error", "")
		return nil, false
	}

	candidates, err := rc.resolveCandidates(meta.Model)
	if err != nil {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, meta.Model, http.StatusBadRequest, err.Error())
		writeRelayError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "")
		return nil, false
	}
	if len(candidates) == 0 {
		rc.logNoChannel(c, requestID, startTime, app, mode, format, meta.Model, http.StatusNotFound, "没有可用的渠道")
		writeRelayError(c, http.StatusNotFound, "没有找到支持该模型的渠道", "invalid_request_error", "")
		return nil, false
	}
	candidates = rc.filterCircuitOpenCandidates(app, candidates)

	return &RequestContext{
		Gin:          c,
		RequestID:    requestID,
		StartTime:    startTime,
		App:          app,
		Mode:         mode,
		Format:       format,
		Method:       c.Request.Method,
		OriginalPath: c.Request.URL.Path,
		Query:        c.Request.URL.RawQuery,
		RawBody:      body,
		Body:         body,
		Meta:         meta,
		Candidates:   candidates,
	}, true
}
