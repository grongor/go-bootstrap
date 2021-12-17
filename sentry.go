package app

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/getsentry/sentry-go"
	goerrors "github.com/go-errors/errors"
	"github.com/grongor/panicwatch"
)

type PanicwatchSentryIntegration struct{}

func (*PanicwatchSentryIntegration) Name() string {
	return "github.com/grongor/panicwatch-sentry"
}

func (p *PanicwatchSentryIntegration) SetupOnce(client *sentry.Client) {
	client.AddEventProcessor(p.processor)
}

func (*PanicwatchSentryIntegration) processor(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	for key, value := range event.Extra {
		panicInfo, ok := value.(panicwatch.Panic)
		if !ok {
			continue
		}

		var panicErr *goerrors.Error
		if !errors.As(panicInfo.AsError(), &panicErr) {
			continue
		}

		event.Level = sentry.LevelFatal
		event.Message = panicInfo.Message

		panicStackFrames := panicErr.StackFrames()
		panicStackFramesCount := len(panicStackFrames)

		frames := make([]sentry.Frame, panicStackFramesCount)

		for i, frame := range panicStackFrames {
			sentryFrame := sentry.Frame{
				Function: frame.Name,
				Package:  frame.Package,
				Lineno:   frame.LineNumber,
			}

			if filepath.IsAbs(frame.File) {
				sentryFrame.AbsPath = frame.File
			} else if frame.File != "" {
				sentryFrame.Filename = frame.File
			}

			frames[panicStackFramesCount-1-i] = sentryFrame
		}

		event.Exception = []sentry.Exception{
			{
				Type:       "panic",
				Value:      panicInfo.Message,
				Stacktrace: &sentry.Stacktrace{Frames: frames},
			},
		}

		delete(event.Extra, key)

		return event
	}

	return event
}

type TrimPathSentryIntegration struct {
	// AppModule is optional (for cases where auto-detection doesn't work) and is used when building with -trimpath.
	// Full module name is expected (optionally ending with slash), eg: github.com/grongor/panicwatch
	AppModule string
	// AppPath is optional (for cases where auto-detection doesn't work) and is used when building without -trimpath.
	// Absolute path to the root module is expected (optionally ending with slash), eg: /myprojects/panicwatch/
	AppPath string
}

func (*TrimPathSentryIntegration) Name() string {
	return "github.com/grongor/go-bootstrap/trim-path-sentry-integration"
}

func (t *TrimPathSentryIntegration) SetupOnce(client *sentry.Client) {
	client.AddEventProcessor(t.processor)

	if t.AppModule != "" {
		if t.AppModule[len(t.AppModule)-1] != '/' {
			t.AppModule += "/"
		}

		return
	} else if t.AppPath != "" {
		if t.AppPath[len(t.AppPath)-1] != '/' {
			t.AppPath += "/"
		}

		return
	}

	t.extractAppPathAndModuleFromStack()
}

func (t *TrimPathSentryIntegration) extractAppPathAndModuleFromStack() {
	buf := make([]byte, 1<<20)

	for {
		if n := runtime.Stack(buf, true); n < len(buf) {
			buf = buf[:n]

			break
		}

		buf = make([]byte, 2*len(buf))
	}

	buf = buf[bytes.Index(buf, []byte("\nmain.main"))+1:]
	buf = buf[bytes.IndexByte(buf, '\n')+1:]
	path := string(bytes.TrimSpace(buf[:bytes.IndexByte(buf, ':')]))

	if !filepath.IsAbs(path) {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			t.AppModule = buildInfo.Main.Path + "/"
		}

		return
	}

	if cmdIndex := strings.LastIndex(path, "/cmd/"); cmdIndex != -1 {
		path = path[0 : cmdIndex+1]
	} else if binIndex := strings.LastIndex(path, "/bin/"); binIndex != -1 {
		path = path[0 : binIndex+1]
	} else {
		path = filepath.Dir(path) + "/"
	}

	if filepath.IsAbs(path) {
		t.AppPath = path
	} else {
		t.AppModule = path
	}
}

func (t *TrimPathSentryIntegration) processor(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	for _, exception := range event.Exception {
		if exception.Stacktrace == nil {
			continue
		}

		t.processStacktrace(exception.Stacktrace)
	}

	for _, thread := range event.Threads {
		if thread.Stacktrace == nil {
			continue
		}

		t.processStacktrace(thread.Stacktrace)
	}

	return event
}

func (t *TrimPathSentryIntegration) processStacktrace(stacktrace *sentry.Stacktrace) {
	for i, frame := range stacktrace.Frames {
		if frame.AbsPath != "" {
			frame.InApp = strings.HasPrefix(frame.AbsPath, t.AppPath)
			frame.AbsPath = strings.TrimPrefix(frame.AbsPath, t.AppPath)
		} else {
			frame.InApp = strings.HasPrefix(frame.Filename, t.AppModule)
			frame.Filename = strings.TrimPrefix(frame.Filename, t.AppModule)
		}

		stacktrace.Frames[i] = frame
	}
}
