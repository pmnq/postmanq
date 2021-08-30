package responsewriter

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"
)

type MonitoringResponseWriter interface {
	GetStartTime() time.Time
	GetCode() int
}

type ResponseWriter struct {
	http.ResponseWriter
	http.Hijacker
	startTime time.Time
	code      int
}

// Wrap takes a normal handler.ResponseWriter and returns our custom ResponseWriter type
func Wrap(rw http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: rw,
		startTime:      time.Now(),
	}
}

func (w ResponseWriter) GetStartTime() time.Time {
	return w.startTime
}

func (w ResponseWriter) GetCode() int {
	return w.code
}

func (w *ResponseWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if ok {
		return h.Hijack()
	}

	return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
}
