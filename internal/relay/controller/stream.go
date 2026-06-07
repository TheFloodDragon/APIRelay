package controller

import (
	"bufio"
	"io"
	"net/http"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) relayStream(c *gin.Context, requestID string, startTime time.Time, mode constant.RelayMode, format constant.RelayFormat, meta relayRequestMeta, body []byte, candidates []relayCandidate) {
	var lastErr error
	var lastErrMsg string
	attemptedUpstream := false

	for _, candidate := range candidates {
		info := buildRelayInfo(c, requestID, startTime, mode, format, meta, candidate, true)
		protocolAdaptor := adaptor.GetAdaptor(info.APIType)

		requestBody, err := bodyWithResolvedModel(body, info.ResolvedModel, format)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, http.StatusBadRequest, lastErrMsg)
			continue
		}

		convertedBody, err := convertRelayRequest(protocolAdaptor, requestBody, info)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			statusCode := http.StatusBadGateway
			if isUnsupportedRelayModeError(err) {
				statusCode = http.StatusBadRequest
			}
			rc.logRequest(c, info, statusCode, lastErrMsg)
			continue
		}

		attemptedUpstream = true
		headers := http.Header{}
		protocolAdaptor.SetupHeaders(headers, info.Channel.APIKey, mode)
		headers.Set("Accept", "text/event-stream")
		url := requestURL(protocolAdaptor, info, true)

		resp, err := rc.httpClient.DoStream(c.Request.Context(), c.Request.Method, url, headers, convertedBody, timeoutForChannel(info.Channel))
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(c, info, 0, lastErrMsg)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errorBody, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
				lastErrMsg = readErr.Error()
			} else {
				lastErr = nil
				lastErrMsg = protocolAdaptor.ErrorMessage(errorBody)
				if lastErrMsg == "" {
					lastErrMsg = string(errorBody)
				}
			}
			rc.logRequest(c, info, resp.StatusCode, lastErrMsg)
			continue
		}

		writeStreamHeaders(c, resp.StatusCode)
		copyErr := copyStream(c, resp.Body, protocolAdaptor, mode, format)
		_ = resp.Body.Close()
		if copyErr != nil {
			lastErr = copyErr
			lastErrMsg = copyErr.Error()
			rc.logRequest(c, info, resp.StatusCode, lastErrMsg)
			return
		}

		rc.logRequest(c, info, resp.StatusCode, "")
		return
	}

	writeFinalRelayError(c, lastErr, lastErrMsg, attemptedUpstream)
}

func writeStreamHeaders(c *gin.Context, statusCode int) {
	writer := c.Writer
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(statusCode)
	writer.Flush()
}

func copyStream(c *gin.Context, body io.Reader, protocolAdaptor adaptor.Adaptor, mode constant.RelayMode, format constant.RelayFormat) error {
	if protocolAdaptor.APIType() == constant.APITypeOpenAI {
		return copyRawStream(c, body, protocolAdaptor, mode, format)
	}
	return copyLineStream(c, body, protocolAdaptor, mode, format)
}

func copyRawStream(c *gin.Context, body io.Reader, protocolAdaptor adaptor.Adaptor, mode constant.RelayMode, format constant.RelayFormat) error {
	buffer := make([]byte, 4096)
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			convertedChunk, err := protocolAdaptor.ConvertStreamChunk(chunk, mode, format)
			if err != nil {
				return err
			}
			if len(convertedChunk) > 0 {
				if _, err := c.Writer.Write(convertedChunk); err != nil {
					return err
				}
				c.Writer.Flush()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

func copyLineStream(c *gin.Context, body io.Reader, protocolAdaptor adaptor.Adaptor, mode constant.RelayMode, format constant.RelayFormat) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		convertedChunk, err := protocolAdaptor.ConvertStreamChunk([]byte(line), mode, format)
		if err != nil {
			return err
		}
		if len(convertedChunk) == 0 {
			continue
		}
		if _, err := c.Writer.Write(convertedChunk); err != nil {
			return err
		}
		c.Writer.Flush()
	}
	return scanner.Err()
}
