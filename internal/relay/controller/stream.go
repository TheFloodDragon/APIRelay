package controller

import (
	"bufio"
	"io"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) relayStream(reqCtx *RequestContext) {
	var lastErr error
	var lastErrMsg string
	attemptedUpstream := false

	for _, candidate := range reqCtx.Candidates {
		attempt, err := rc.buildRelayAttempt(reqCtx, candidate, true)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			statusCode := relayAttemptErrorStatus(err, http.StatusBadGateway)
			if attempt != nil {
				rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, lastErrMsg)
			}
			continue
		}

		attemptedUpstream = true
		resp, err := rc.httpClient.DoStream(
			reqCtx.Gin.Request.Context(),
			reqCtx.Method,
			attempt.URL,
			attempt.Headers,
			attempt.ConvertedBody,
			timeoutForChannel(attempt.Info.Channel),
		)
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(reqCtx.Gin, attempt.Info, 0, lastErrMsg)
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
				lastErrMsg = attempt.ProtocolAdaptor.ErrorMessage(errorBody)
				if lastErrMsg == "" {
					lastErrMsg = string(errorBody)
				}
			}
			rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, lastErrMsg)
			continue
		}

		preparedBody, err := prepareStreamBody(reqCtx.Gin.Request.Context(), resp.Body, timeoutForChannel(attempt.Info.Channel))
		if err != nil {
			lastErr = err
			lastErrMsg = err.Error()
			rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, lastErrMsg)
			continue
		}

		writeStreamHeaders(reqCtx.Gin, resp.StatusCode)
		copyErr := copyStream(reqCtx.Gin, preparedBody, attempt.ProtocolAdaptor, reqCtx.Mode, reqCtx.Format)
		_ = preparedBody.Close()
		if copyErr != nil {
			lastErr = copyErr
			lastErrMsg = copyErr.Error()
			rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, lastErrMsg)
			return
		}

		rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, "")
		return
	}

	writeFinalRelayError(reqCtx.Gin, lastErr, lastErrMsg, attemptedUpstream)
}

func writeStreamHeaders(c *gin.Context, statusCode int) {
	writer := c.Writer
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
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
