package controller

import "io"

// relayResponseBody 按 RelayAttempt 的转换标记处理非流式响应。
// 透传路径直接返回上游原始 body，避免再次解析/序列化。
func relayResponseBody(attempt *RelayAttempt, respBody []byte) ([]byte, error) {
	if attempt == nil || attempt.ProtocolAdaptor == nil || !attempt.NeedsTransform {
		return respBody, nil
	}
	return attempt.ProtocolAdaptor.ConvertResponse(respBody, attempt.Info.RelayMode, attempt.Info.RelayFormat)
}

// copyPassthroughStream 直接复制上游响应流到客户端 writer。
func copyPassthroughStream(dst io.Writer, src io.Reader, flush func()) error {
	buffer := make([]byte, 4096)
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, err := dst.Write(buffer[:n]); err != nil {
				return err
			}
			if flush != nil {
				flush()
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
