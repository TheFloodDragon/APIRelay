package response

import (
	"bufio"
	"io"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
}

func ParseSSE(r io.Reader) ([]SSEEvent, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	events := make([]SSEEvent, 0)
	currentEvent := ""
	dataLines := make([]string, 0)

	flush := func() {
		if currentEvent == "" && len(dataLines) == 0 {
			return
		}
		events = append(events, SSEEvent{Event: currentEvent, Data: strings.Join(dataLines, "\n")})
		currentEvent = ""
		dataLines = dataLines[:0]
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	return events, scanner.Err()
}

func WriteSSEEvent(w io.Writer, eventName string, data []byte) error {
	if eventName != "" {
		if _, err := w.Write([]byte("event: " + eventName + "\n")); err != nil {
			return err
		}
	}
	if len(data) > 0 {
		if _, err := w.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("\n"))
	return err
}
