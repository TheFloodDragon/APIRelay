package controller

import (
	"bufio"
	"io"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/adaptor"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	relayresponse "github.com/TheFloodDragon/APIRelay/internal/relay/response"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) relayStream(reqCtx *RequestContext) {
	resp, attempt, err := rc.forwardRelayAttempt(
		reqCtx,
		true,
		func(provider model.Channel) (*RelayAttempt, error) {
			candidate, ok := reqCtx.candidateForProvider(provider.ID)
			if !ok {
				return nil, newNonRetryableBuildError(http.StatusNotFound, "provider does not support requested model")
			}
			candidate.Channel = provider
			return rc.buildRelayAttempt(reqCtx, candidate, true)
		},
		relayStreamPreflight,
	)
	if err != nil {
		statusCode, errMsg := relayFailureDetails(err, attempt)
		if attempt != nil {
			rc.logRequest(reqCtx.Gin, attempt.Info, statusCode, errMsg)
		}
		writeFinalRelayError(reqCtx.Gin, err, errMsg, attempt != nil)
		return
	}

	writeStreamHeaders(reqCtx.Gin, resp.StatusCode, resp.Header)
	copyErr := copyAttemptStream(reqCtx.Gin, resp.Body, attempt)
	_ = resp.Body.Close()
	if copyErr != nil {
		errMsg := copyErr.Error()
		if rc.providerRouter != nil {
			rc.providerRouter.RecordFailure(attempt.Info.Channel.ID, errMsg)
		}
		rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, errMsg)
		return
	}

	if rc.providerRouter != nil {
		rc.providerRouter.RecordSuccess(attempt.Info.Channel.ID)
	}
	rc.logRequest(reqCtx.Gin, attempt.Info, resp.StatusCode, "")
}

func writeStreamHeaders(c *gin.Context, statusCode int, headers http.Header) {
	writer := c.Writer
	relayresponse.WriteStreamHeaders(writer, statusCode, headers)
	writer.Flush()
}

func copyAttemptStream(c *gin.Context, body io.Reader, attempt *RelayAttempt) error {
	if attempt == nil {
		return copyPassthroughStream(c.Writer, body, c.Writer.Flush)
	}
	if !attempt.NeedsTransform {
		return copyPassthroughStream(c.Writer, body, c.Writer.Flush)
	}
	return copyStream(c, body, attempt.ProtocolAdaptor, attempt.Info.RelayMode, attempt.Info.RelayFormat)
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
