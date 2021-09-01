package app

import (
	"context"
	"os"

	"github.com/getsentry/sentry-go"
	"github.com/grongor/panicwatch"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var (
	Release string

	instance = &Application{}
)

type Application struct {
	started            atomic.Bool
	terminationSignals []os.Signal
	extensions         []Extension
	sentryConfig       func(*sentry.ClientOptions)
	zapConfig          func(*zap.Config)
	panicwatchConfig   func(*panicwatch.Config, *zap.SugaredLogger)

	config *genericConfig
}

func (app *Application) WithConfig(config interface{}) *Application {
	app.checkStarted(false)

	return app.WithExtensions(&appConfigExtension{config: config})
}

func (app *Application) WithTerminationSignals(signals ...os.Signal) *Application {
	app.checkStarted(false)

	app.terminationSignals = append(app.terminationSignals, signals...)

	return app
}

func (app *Application) WithExtensions(extensions ...Extension) *Application {
	app.checkStarted(false)

	app.extensions = append(app.extensions, extensions...)

	return app
}

func (app *Application) WithZapConfig(zapConfig func(*zap.Config)) *Application {
	app.checkStarted(false)

	app.zapConfig = zapConfig

	return app
}

func (app *Application) WithSentryConfig(sentryConfig func(*sentry.ClientOptions)) *Application {
	app.checkStarted(false)

	app.sentryConfig = sentryConfig

	return app
}

func (app *Application) WithPanicwatchConfig(
	panicwatchConfig func(*panicwatch.Config, *zap.SugaredLogger),
) *Application {
	app.checkStarted(false)

	app.panicwatchConfig = panicwatchConfig

	return app
}

func (app *Application) Run(appCallback func(appCtx context.Context, logger *zap.SugaredLogger)) {
	app.checkStarted(true)

	app.config = &genericConfig{}

	app.config.Load(app.extensions, app.zapConfig, app.sentryConfig, app.panicwatchConfig)

	logger := app.config.logger
	defer logger.Sync()

	appCtx := NewCliContext(app.terminationSignals...)

	for _, extension := range app.extensions {
		extension.Start(appCtx, logger)
	}

	appFinishedCh := make(chan struct{})

	appCtx.StartWorker(func() {
		logger.Info("running the application")
		appCallback(appCtx.ctx, logger)
		close(appFinishedCh)
		appCtx.Shutdown()
	})

	signalLoggerCh := make(chan struct{})

	go func() {
		select {
		case <-appFinishedCh:
		case <-appCtx.Done():
			select {
			case <-appFinishedCh:
			default:
				logger.Info("received termination signal, application should finish soon")
			}
		}

		close(signalLoggerCh)
	}()

	appCtx.WaitForWorkers()

	<-signalLoggerCh

	logger.Info("application finished, exiting now")
}

func (app *Application) Start(appCallback func(appCtx Context, logger *zap.SugaredLogger)) {
	app.checkStarted(true)

	app.config = &genericConfig{}

	app.config.Load(app.extensions, app.zapConfig, app.sentryConfig, app.panicwatchConfig)

	logger := app.config.logger
	defer logger.Sync()

	appCtx := NewCliContext(app.terminationSignals...)

	for _, extension := range app.extensions {
		extension.Start(appCtx, logger)
	}

	appCtx.StartWorker(func() {
		appCallback(appCtx, logger)
		logger.Info("application started")
	})

	shutdownLoggerCh := make(chan struct{})

	appCtx.StartWorker(func() {
		<-appCtx.Done()

		logger.Info("application is shutting down")

		close(shutdownLoggerCh)
	})

	appCtx.WaitForWorkers()

	<-shutdownLoggerCh

	logger.Info("application finished, exiting now")
}

func (app *Application) checkStarted(doStart bool) {
	if doStart {
		if app.started.CAS(false, true) {
			return
		}
	} else if !app.started.Load() {
		return
	}

	panic("application has already been started")
}

func WithConfig(config interface{}) *Application {
	return instance.WithConfig(config)
}

func WithTerminationSignals(signals ...os.Signal) *Application {
	return instance.WithTerminationSignals(signals...)
}

func WithExtensions(extensions ...Extension) *Application {
	return instance.WithExtensions(extensions...)
}

func WithZapConfig(zapConfig func(*zap.Config)) *Application {
	return instance.WithZapConfig(zapConfig)
}

func WithSentryConfig(sentryConfig func(*sentry.ClientOptions)) *Application {
	return instance.WithSentryConfig(sentryConfig)
}

func WithPanicwatchConfig(panicwatchConfig func(*panicwatch.Config, *zap.SugaredLogger)) *Application {
	return instance.WithPanicwatchConfig(panicwatchConfig)
}

func Run(appCallback func(appCtx context.Context, logger *zap.SugaredLogger)) {
	instance.Run(appCallback)
}

func Start(appCallback func(appCtx Context, logger *zap.SugaredLogger)) {
	instance.Start(appCallback)
}
