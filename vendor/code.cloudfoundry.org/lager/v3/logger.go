package lager

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/openzipkin/zipkin-go/idgenerator"
	"github.com/openzipkin/zipkin-go/model"
)

const (
	StackTraceBufferSize = 1024 * 100
	RequestIdHeader      = "X-Vcap-Request-Id"
)

type Logger interface {
	RegisterSink(Sink)
	Session(task string, data ...Data) Logger
	SessionName() string
	Debug(action string, data ...Data)
	Info(action string, data ...Data)
	Error(action string, err error, data ...Data)
	Fatal(action string, err error, data ...Data)
	WithData(Data) Logger
	WithTraceInfo(*http.Request) Logger
}

type logger struct {
	component   string
	task        string
	sinks       []Sink
	sessionID   string
	nextSession uint32
	data        Data
}

func NewLogger(component string) Logger {
	return &logger{
		component: component,
		task:      component,
		sinks:     []Sink{},
		data:      Data{},
	}
}

func (l *logger) RegisterSink(sink Sink) {
	l.sinks = append(l.sinks, sink)
}

func (l *logger) SessionName() string {
	return l.task
}

func (l *logger) Session(task string, data ...Data) Logger {
	sid := atomic.AddUint32(&l.nextSession, 1)

	var sessionIDstr string

	if l.sessionID != "" {
		sessionIDstr = fmt.Sprintf("%s.%d", l.sessionID, sid)
	} else {
		sessionIDstr = fmt.Sprintf("%d", sid)
	}

	return &logger{
		component: l.component,
		task:      fmt.Sprintf("%s.%s", l.task, task),
		sinks:     l.sinks,
		sessionID: sessionIDstr,
		data:      l.baseData(data...),
	}
}

func (l *logger) WithData(data Data) Logger {
	return &logger{
		component: l.component,
		task:      l.task,
		sinks:     l.sinks,
		sessionID: l.sessionID,
		data:      l.baseData(data),
	}
}

func (l *logger) WithTraceInfo(req *http.Request) Logger {
	traceIDHeader := req.Header.Get(RequestIdHeader)
	if traceIDHeader == "" {
		return l.WithData(nil)
	}
	traceHex := strings.Replace(traceIDHeader, "-", "", -1)
	traceID, err := model.TraceIDFromHex(traceHex)
	if err != nil {
		return l.WithData(nil)
	}

	spanID := idgenerator.NewRandom128().SpanID(traceID)
	return l.WithData(Data{"trace-id": traceID.String(), "span-id": spanID.String()})
}

func (l *logger) Debug(action string, data ...Data) {
	t := time.Now().UTC()
	log := LogFormat{
		time:      t,
		Timestamp: formatTimestamp(t),
		Source:    l.component,
		Message:   fmt.Sprintf("%s.%s", l.task, action),
		LogLevel:  DEBUG,
		Data:      l.baseData(data...),
	}

	for _, sink := range l.sinks {
		sink.Log(log)
	}
}

func (l *logger) Info(action string, data ...Data) {
	t := time.Now().UTC()
	log := LogFormat{
		time:      t,
		Timestamp: formatTimestamp(t),
		Source:    l.component,
		Message:   fmt.Sprintf("%s.%s", l.task, action),
		LogLevel:  INFO,
		Data:      l.baseData(data...),
	}

	for _, sink := range l.sinks {
		sink.Log(log)
	}
}

func (l *logger) Error(action string, err error, data ...Data) {
	logData := l.baseData(data...)

	if err != nil {
		logData["error"] = err.Error()
	}

	t := time.Now().UTC()
	log := LogFormat{
		time:      t,
		Timestamp: formatTimestamp(t),
		Source:    l.component,
		Message:   fmt.Sprintf("%s.%s", l.task, action),
		LogLevel:  ERROR,
		Data:      logData,
		Error:     err,
	}

	for _, sink := range l.sinks {
		sink.Log(log)
	}
}

func (l *logger) Fatal(action string, err error, data ...Data) {
	logData := l.baseData(data...)

	stackTrace := make([]byte, StackTraceBufferSize)
	stackSize := runtime.Stack(stackTrace, false)
	stackTrace = stackTrace[:stackSize]

	if err != nil {
		logData["error"] = err.Error()
	}

	logData["trace"] = string(stackTrace)

	t := time.Now().UTC()
	log := LogFormat{
		time:      t,
		Timestamp: formatTimestamp(t),
		Source:    l.component,
		Message:   fmt.Sprintf("%s.%s", l.task, action),
		LogLevel:  FATAL,
		Data:      logData,
		Error:     err,
	}

	for _, sink := range l.sinks {
		sink.Log(log)
	}

	panic(err)
}

func (l *logger) baseData(givenData ...Data) Data {
	data := Data{}

	for k, v := range l.data {
		data[k] = v
	}

	if len(givenData) > 0 {
		for _, dataArg := range givenData {
			for key, val := range dataArg {
				data[key] = val
			}
		}
	}

	if l.sessionID != "" {
		data["session"] = l.sessionID
	}

	return data
}

func formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%.9f", float64(t.UnixNano())/1e9)
}
