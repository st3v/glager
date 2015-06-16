package glager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/types"
	"github.com/pivotal-golang/lager"
)

type logEntry lager.LogFormat

type logEntries []logEntry

type logEntryData lager.Data

type option func(*logEntry)

type TestLogger struct {
	lager.Logger
	buf *gbytes.Buffer
}

func NewLogger(component string) *TestLogger {
	buf := gbytes.NewBuffer()
	log := lager.NewLogger(component)
	log.RegisterSink(lager.NewWriterSink(buf, lager.DEBUG))
	return &TestLogger{log, buf}
}

func (l *TestLogger) Buffer() *gbytes.Buffer {
	return l.buf
}

type logMatcher struct {
	actual   logEntries
	expected logEntries
}

func ContainSequence(expectedSequence ...logEntry) types.GomegaMatcher {
	return &logMatcher{
		expected: expectedSequence,
	}
}

func Info(options ...option) logEntry {
	return Entry(lager.INFO, options...)
}

func Debug(options ...option) logEntry {
	return Entry(lager.DEBUG, options...)
}

var AnyErr error = nil

func Error(err error, options ...option) logEntry {
	if err != nil {
		options = append(options, Data("error", err.Error()))
	}

	return Entry(lager.ERROR, options...)
}

func Fatal(err error, options ...option) logEntry {
	if err != nil {
		options = append(options, Data("error", err.Error()))
	}

	return Entry(lager.FATAL, options...)
}

func Entry(logLevel lager.LogLevel, options ...option) logEntry {
	entry := logEntry(lager.LogFormat{
		LogLevel: logLevel,
		Data:     lager.Data{},
	})

	for _, option := range options {
		option(&entry)
	}

	return entry
}

func Message(msg string) option {
	return func(e *logEntry) {
		e.Message = msg
	}
}

func Action(action string) option {
	return Message(action)
}

func Source(src string) option {
	return func(e *logEntry) {
		e.Source = src
	}
}

func Data(kv ...interface{}) option {
	if len(kv)%2 == 1 {
		kv = append(kv, "")
	}

	return func(e *logEntry) {
		for i := 0; i < len(kv); i += 2 {
			key, ok := kv[i].(string)
			if !ok {
				err := fmt.Errorf("Invalid type for data key. Want string. Got %T:%v.", kv[i], kv[i])
				panic(err)
			}
			e.Data[key] = kv[i+1]
		}
	}
}

type ContentsProvider interface {
	Contents() []byte
}

func (lm *logMatcher) Match(actual interface{}) (success bool, err error) {
	var reader io.Reader

	switch x := actual.(type) {
	case gbytes.BufferProvider:
		reader = bytes.NewReader(x.Buffer().Contents())
	case ContentsProvider:
		reader = bytes.NewReader(x.Contents())
	case io.Reader:
		reader = x
	default:
		return false, fmt.Errorf("ContainSequence must be passed an io.Reader, glager.ContentsProvider, or gbytes.BufferProvider. Got:\n%s", format.Object(actual, 1))
	}

	decoder := json.NewDecoder(reader)

	lm.actual = logEntries{}

	for {
		var entry logEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return false, err
		}
		lm.actual = append(lm.actual, entry)
	}

	actualEntries := lm.actual

	for _, expected := range lm.expected {
		i, found, err := actualEntries.indexOf(expected)
		if err != nil {
			return false, err
		}

		if !found {
			return false, nil
		}

		actualEntries = actualEntries[i+1:]
	}

	return true, nil
}

func (lm *logMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%s\nto contain log sequence \n\t%s",
		format.Object(lm.actual, 0),
		format.Object(lm.expected, 0),
	)
}

func (lm *logMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%s\nnot to contain log sequence \n\t%s",
		format.Object(lm.actual, 0),
		format.Object(lm.expected, 0),
	)
}

func (entry logEntry) logData() logEntryData {
	return logEntryData(entry.Data)
}

func (actual logEntry) contains(expected logEntry) (bool, error) {
	if expected.Source != "" && actual.Source != expected.Source {
		return false, nil
	}

	if expected.Message != "" && actual.Message != expected.Message {
		return false, nil
	}

	if actual.LogLevel != expected.LogLevel {
		return false, nil
	}

	containsData, err := actual.logData().contains(expected.logData())
	if err != nil {
		return false, err
	}

	return containsData, nil
}

func (actual logEntryData) contains(expected logEntryData) (bool, error) {
	for expectedKey, expectedVal := range expected {
		actualVal, found := actual[expectedKey]
		if !found {
			return false, nil
		}

		// this has been marshalled and unmarshalled before, no need to check err
		actualJSON, _ := json.Marshal(actualVal)

		expectedJSON, err := json.Marshal(expectedVal)
		if err != nil {
			return false, err
		}

		if string(actualJSON) != string(expectedJSON) {
			return false, nil
		}
	}
	return true, nil
}

func (entries logEntries) indexOf(entry logEntry) (int, bool, error) {
	for i, actual := range entries {
		containsEntry, err := actual.contains(entry)
		if err != nil {
			return 0, false, err
		}

		if containsEntry {
			return i, true, nil
		}
	}
	return 0, false, nil
}
