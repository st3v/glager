package glager

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/pivotal-golang/lager"
)

func ContainSequence(expectedSequence ...LogEntry) types.GomegaMatcher {
	return &logMatcher{
		expected: expectedSequence,
	}
}

func Info(options ...option) LogEntry {
	return Entry(lager.INFO, options...)
}

func Debug(options ...option) LogEntry {
	return Entry(lager.DEBUG, options...)
}

func Error(err error, options ...option) LogEntry {
	options = append(options, Data("error", err.Error()))
	return Entry(lager.ERROR, options...)
}

func Fatal(options ...option) LogEntry {
	return Entry(lager.FATAL, options...)
}

func Entry(logLevel lager.LogLevel, options ...option) LogEntry {
	entry := LogEntry(lager.LogFormat{
		LogLevel: logLevel,
		Data:     lager.Data{},
	})

	for _, option := range options {
		option(&entry)
	}

	return entry
}

type LogEntry lager.LogFormat

type LogEntryData lager.Data

type option func(*LogEntry)

func Message(msg string) option {
	return func(e *LogEntry) {
		e.Message = msg
	}
}

func Action(action string) option {
	return Message(action)
}

func Source(src string) option {
	return func(e *LogEntry) {
		e.Source = src
	}
}

func Data(kv ...string) option {
	if len(kv)%2 == 1 {
		kv = append(kv, "")
	}

	return func(e *LogEntry) {
		for i := 0; i < len(kv); i += 2 {
			e.Data[kv[i]] = kv[i+1]
		}
	}
}

type logMatcher struct {
	actual   LogEntries
	expected LogEntries
}

func (lm *logMatcher) Match(actual interface{}) (success bool, err error) {
	reader, ok := actual.(io.Reader)
	if !ok {
		return false, fmt.Errorf("Contains must be passed an io.Reader. Got:\n%s", format.Object(actual, 1))
	}

	decoder := json.NewDecoder(reader)

	lm.actual = LogEntries{}

	for {
		var entry LogEntry
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			return false, err
		}
		lm.actual = append(lm.actual, entry)
	}

	actualEntries := lm.actual

	for _, expected := range lm.expected {
		i, found := actualEntries.indexOf(expected)

		if !found {
			return false, nil
		}

		actualEntries = actualEntries[i+1:]
	}

	return true, nil
}

type LogEntries []LogEntry

func (entries LogEntries) indexOf(entry LogEntry) (int, bool) {
	for i, actual := range entries {
		if actual.contains(entry) {
			return i, true
		}
	}
	return 0, false
}

func (entries LogEntries) contains(entry LogEntry) bool {
	for _, actual := range entries {
		if actual.contains(entry) {
			return true
		}
	}
	return false
}

func (entry LogEntry) LogData() LogEntryData {
	return LogEntryData(entry.Data)
}

func (actual LogEntry) contains(expected LogEntry) bool {
	if expected.Source != "" && actual.Source != expected.Source {
		return false
	}

	if expected.Message != "" && actual.Message != expected.Message {
		return false
	}

	if actual.LogLevel != expected.LogLevel {
		return false
	}

	if expected.Timestamp != "" && actual.Timestamp != expected.Timestamp {
		return false
	}

	if !actual.LogData().contains(expected.LogData()) {
		return false
	}

	return true
}

func (actual LogEntryData) contains(expected LogEntryData) bool {
	for k, v := range expected {
		actualValue, found := actual[k]
		if !found || v != actualValue {
			return false
		}
	}
	return true
}

func (lm *logMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto contain log sequence \n\t%#v",
		lm.actual,
		lm.expected,
	)
}

func (lm *logMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nnot to contain log sequence \n\t%#v",
		lm.actual,
		lm.expected,
	)
}

/**

Expect(log).To(ContainLagerEntries(
	lager.LogFormat{
		Message: "logger.pipeline-step",
		Source:  "loger",
		Data: lager.Data{
			"event":    "done",
			"pipeline": "some-name",
			"task":     "task1",
		},
	},
))

Expect(log).To(glager.Log(
	lager.LogFormat{
		Message: "logger.pipeline-step",
		Source:  "loger",
		Data: lager.Data{
			"event":    "done",
			"pipeline": "some-name",
			"task":     "task1",
		},
	},
))

Expect(log).To(glager.Log(
	glager.Entry(
		glager.Message("logger.pipeline-step"),
		glager.Source("loger"),
		glager.Data("event", "starting"),
		glager.Data("task", "task1"),
	),
	glager.Entry(
		glager.Message("loger.pipeline-step")
		glager.Source("loger"),
		glager.Data("event", "done"),
		glager.Data("task", "task1"),
	),
))

Expect(log).To(glager.Log(
	glager.Entry(
		glager.Message("logger.pipeline-step"),
		glager.Source("loger"),
		glager.Data("event", "starting", "task", "task1"),
	),
	glager.Entry(
		glager.Message("loger.pipeline-step")
		glager.Source("loger"),
		glager.Data("event", "done", "task", "task1"),
	),
))
*/
