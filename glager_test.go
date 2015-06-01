package glager_test

import (
	"bufio"
	"errors"
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	. "github.com/st3v/glager"
)

func TestGlager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Glager Test Suite")
}

var _ = Describe(".ContainSequence", func() {
	var (
		buffer *gbytes.Buffer
		logger lager.Logger
	)

	BeforeEach(func() {
		buffer = gbytes.NewBuffer()
		logger = lager.NewLogger("logger")
		logger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))
	})

	Context("when actual is a BufferProvider", func() {
		var sink *lagertest.TestSink

		BeforeEach(func() {
			sink = lagertest.NewTestSink()
			logger.RegisterSink(sink)
			logger.Info("foo")
		})

		It("matches an entry", func() {
			Expect(sink).To(ContainSequence(Info()))
		})

		It("does match on subsequent calls", func() {
			Expect(sink).To(ContainSequence(Info()))
			Expect(sink).To(ContainSequence(Info()))
		})
	})

	Context("when actual is a ContentsProvider", func() {
		BeforeEach(func() {
			logger.Info("foo")
		})

		It("matches an entry", func() {
			Expect(buffer).To(ContainSequence(Info()))
		})

		It("does match on subsequent calls", func() {
			Expect(buffer).To(ContainSequence(Info()))
			Expect(buffer).To(ContainSequence(Info()))
		})
	})

	Context("when actual is an io.Reader", func() {
		var log io.Reader

		Context("containing a valid lager error log entry", func() {
			var expectedError = errors.New("some-error")

			BeforeEach(func() {
				buffer := gbytes.NewBuffer()
				logger := lager.NewLogger("logger")
				logger.RegisterSink(lager.NewWriterSink(buffer, lager.DEBUG))

				logger.Info("action", lager.Data{"event": "starting", "task": "my-task"})
				logger.Debug("action", lager.Data{"event": "debugging", "task": "my-task"})
				logger.Error("action", expectedError, lager.Data{"event": "failed", "task": "my-task"})

				log = bufio.NewReader(buffer)
			})

			It("does not match on subsequent calls", func() {
				Expect(log).To(ContainSequence(Error(expectedError)))
				Expect(log).ToNot(ContainSequence(Error(expectedError)))
			})

			It("matches an info entry", func() {
				Expect(log).To(ContainSequence(
					Info(
						Source("logger"),
						Message("logger.action"),
						Data("event", "starting"),
						Data("task", "my-task"),
					),
				))
			})

			It("matches a debug entry", func() {
				Expect(log).To(ContainSequence(
					Debug(
						Source("logger"),
						Message("logger.action"),
						Data("event", "debugging"),
						Data("task", "my-task"),
					),
				))
			})

			It("matches an error entry", func() {
				Expect(log).To(ContainSequence(
					Error(
						expectedError,
						Source("logger"),
						Message("logger.action"),
						Data("event", "failed"),
						Data("task", "my-task"),
					),
				))
			})

			It("does match a correct sequence", func() {
				Expect(log).To(ContainSequence(
					Info(
						Data("event", "starting", "task", "my-task"),
					),
					Debug(
						Data("event", "debugging", "task", "my-task"),
					),
					Error(
						expectedError,
						Data("event", "failed", "task", "my-task"),
					),
				))
			})

			It("does not match an incorrect sequence", func() {
				Expect(log).ToNot(ContainSequence(
					Info(
						Data("event", "starting", "task", "my-task"),
					),
					Info(
						Data("event", "starting", "task", "my-task"),
					),
				))
			})

			It("does not match an out-of-order sequence", func() {
				Expect(log).ToNot(ContainSequence(
					Debug(
						Data("event", "debugging", "task", "my-task"),
					),
					Error(
						expectedError,
						Data("event", "failed", "task", "my-task"),
					),
					Info(
						Data("event", "starting", "task", "my-task"),
					),
				))
			})

			It("does not match a fatal entry", func() {
				Expect(log).ToNot(ContainSequence(
					Fatal(
						Source("logger"),
						Data("event", "failed", "task", "my-task"),
					),
				))
			})
		})
	})
})
