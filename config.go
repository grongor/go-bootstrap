package app

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/TheZeroSlave/zapsentry"
	"github.com/getsentry/sentry-go"
	"github.com/grongor/panicwatch"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type ConfigValidator interface {
	Validate() error
}

type genericConfig struct {
	App struct {
		Logger struct {
			Colors bool
			Debug  bool
			Json   bool
			Sentry struct {
				Dsn         string
				Environment string
			}
		}
		LocalTime  bool
		Panicwatch bool
	}
	logger *zap.SugaredLogger
	appCtx *CliContext
}

func (c *genericConfig) Load(
	extensions []Extension,
	zapConfig func(*zap.Config),
	sentryClientOptions func(*sentry.ClientOptions),
	panicwatchConfig func(*panicwatch.Config, *zap.SugaredLogger),
) {
	configBytes, err := c.readConfigFile()
	if err != nil {
		panic("failed to read config file: " + err.Error())
	}

	config := make(map[string]interface{})

	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		panic("failed to unmarshal YAML config file: " + err.Error())
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Result: c,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToTimeHookFunc(time.RFC3339),
			mapstructure.StringToIPHookFunc(),
			mapstructure.StringToIPNetHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
		),
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		panic("failed to create mapstructure.Decoder: " + err.Error())
	}

	if err = decoder.Decode(config); err != nil {
		panic("failed to decode config into the config struct: " + err.Error())
	}

	if !c.App.LocalTime {
		time.Local = time.UTC
	}

	c.setupLogger(zapConfig, sentryClientOptions)

	if c.App.Panicwatch {
		c.startPanicwatch(panicwatchConfig)
	}

	c.initializeExtensions(extensions, decoderConfig, config)
}

func (c *genericConfig) initializeExtensions(
	extensions []Extension,
	decoderConfig *mapstructure.DecoderConfig,
	config map[string]interface{},
) {
	var baseDecodeHook mapstructure.DecodeHookFunc

	if globalHooks := c.getGlobalConfigDecodeHooks(extensions); len(globalHooks) != 0 {
		baseDecodeHook = mapstructure.ComposeDecodeHookFunc(append(globalHooks, decoderConfig.DecodeHook)...)
	} else {
		baseDecodeHook = decoderConfig.DecodeHook
	}

	for _, extension := range extensions {
		err := extension.Initialize(
			func(extensionConfig interface{}, decodeHooks ...mapstructure.DecodeHookFunc) {
				if extensionConfig == nil {
					c.logger.Panic("app extension config is nil; don't call configLoader if you don't use any config")
				}

				if len(decodeHooks) != 0 {
					decodeHooks = append(decodeHooks, baseDecodeHook)
					decoderConfig.DecodeHook = mapstructure.ComposeDecodeHookFunc(decodeHooks...)
				} else {
					decoderConfig.DecodeHook = baseDecodeHook
				}

				decoderConfig.Result = extensionConfig

				decoder, err := mapstructure.NewDecoder(decoderConfig)
				if err != nil {
					c.logger.Fatalw("failed to create mapstructure.Decoder for app extension", zap.Error(err))
				}

				if err = decoder.Decode(config); err != nil {
					c.logger.Fatalw("failed to unmarshal app extension config", zap.Error(err))
				}

				if extensionConfig, ok := extensionConfig.(ConfigValidator); ok {
					if err = extensionConfig.Validate(); err != nil {
						isConfigExtension := true
						logger := c.logger

						if _, ok := extension.(*appConfigExtension); !ok {
							isConfigExtension = false
							logger = logger.With(zap.Stringer("extension", reflect.TypeOf(extension)))
						}

						if errs := multierr.Errors(err); len(errs) > 1 {
							if isConfigExtension {
								logger.Fatalw("config validation failed", "errors", errs)
							} else {
								logger.Fatalw("app extension config validation failed", "errors", errs)
							}
						}

						if isConfigExtension {
							logger.Fatalw("config validation failed", zap.Error(err))
						} else {
							logger.Fatalw("app extension config validation failed", zap.Error(err))
						}
					}
				}
			},
			c.logger,
		)
		if err != nil {
			c.logger.Fatalw("failed to initialize app extension", zap.Error(err))
		}
	}
}

func (c *genericConfig) getGlobalConfigDecodeHooks(extensions []Extension) []mapstructure.DecodeHookFunc {
	var hooks []mapstructure.DecodeHookFunc

	for _, extension := range extensions {
		hooks = append(hooks, extension.GlobalConfigDecodeHooks()...)
	}

	return hooks
}

func (*genericConfig) readConfigFile() ([]byte, error) {
	configFile := flag.String("config", "", "path to a YAML config file")

	flag.Parse()

	if *configFile == "" {
		*configFile = "config.yaml"
	}

	bytes, err := os.ReadFile(*configFile)
	if err != nil && !filepath.IsAbs(*configFile) {
		var executablePath string

		executablePath, err = os.Executable()
		if err != nil {
			panic("failed to determine the executable path: " + err.Error())
		}

		bytes, err = os.ReadFile(filepath.Join(filepath.Dir(executablePath), *configFile))
	}

	return bytes, err
}

func (c *genericConfig) setupLogger(zapConfig func(*zap.Config), sentryConfig func(*sentry.ClientOptions)) {
	loggerConfig := zap.NewProductionConfig()

	loggerConfig.Sampling = nil
	loggerConfig.EncoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		if !c.App.LocalTime && t.Location() != time.UTC {
			t = t.UTC()
		}

		encoder.AppendString(t.Format("2006-01-02T15:04:05.000"))
	}
	loggerConfig.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	loggerConfig.DisableCaller = true
	loggerConfig.DisableStacktrace = true

	if !c.App.Logger.Json {
		loggerConfig.Encoding = "console"

		if c.App.Logger.Colors {
			loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			loggerConfig.EncoderConfig.EncodeName = func(name string, encoder zapcore.PrimitiveArrayEncoder) {
				encoder.AppendString("\x1b[36m" + name + "\x1b[0m")
			}
		} else {
			loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		}
	}

	if c.App.Logger.Debug {
		loggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	if zapConfig != nil {
		zapConfig(&loggerConfig)
	}

	logLevel = &loggerConfig.Level

	logger, err := loggerConfig.Build()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}

	if c.App.Logger.Sentry.Dsn == "" {
		logger.Warn("Sentry DSN is not set; Sentry integration is disabled")
	} else {
		logger = c.setupSentryLogging(sentryConfig, logger)
	}

	c.logger = logger.Sugar()
}

func (c *genericConfig) setupSentryLogging(sentryConfig func(*sentry.ClientOptions), logger *zap.Logger) *zap.Logger {
	config := zapsentry.Configuration{Level: zapcore.WarnLevel}

	options := sentry.ClientOptions{
		Dsn:              c.App.Logger.Sentry.Dsn,
		AttachStacktrace: true,
		Environment:      c.App.Logger.Sentry.Environment,
		Release:          Release,
	}

	filterIntegrations := func(integrations []sentry.Integration) []sentry.Integration {
		filtered := make([]sentry.Integration, 0, len(integrations))

		for _, integration := range integrations {
			if integration.Name() == "ContextifyFrames" {
				continue
			}

			filtered = append(filtered, integration)
		}

		return filtered
	}

	if c.App.Panicwatch {
		options.Integrations = func(integrations []sentry.Integration) []sentry.Integration {
			return append(
				filterIntegrations(integrations),
				&PanicwatchSentryIntegration{},
				&TrimPathSentryIntegration{},
			)
		}
	} else {
		options.Integrations = func(integrations []sentry.Integration) []sentry.Integration {
			return append(filterIntegrations(integrations), &TrimPathSentryIntegration{})
		}
	}

	if sentryConfig != nil {
		sentryConfig(&options)
	}

	if err := sentry.Init(options); err != nil {
		logger.Fatal("failed to initialize Sentry SDK", zap.Error(err))
	}

	core, err := zapsentry.NewCore(config, zapsentry.NewSentryClientFromClient(sentry.CurrentHub().Client()))
	if err != nil {
		logger.Fatal("failed to initialize zapsentry core", zap.Error(err))
	}

	return zapsentry.AttachCoreToLogger(core, logger)
}

func (c *genericConfig) startPanicwatch(panicwatchConfig func(*panicwatch.Config, *zap.SugaredLogger)) {
	logger := c.logger.Named("panicwatch")

	config := panicwatch.Config{
		OnPanic: func(p panicwatch.Panic) {
			logger.Fatalw(p.Message, "panic", p)
		},
		OnWatcherError: func(err error) {
			logger.Errorw("watcher error", zap.Error(err))
			c.appCtx.Shutdown()
		},
		OnWatcherDied: func(err error) {
			logger.Errorw("watcher died", zap.Error(err))
			c.appCtx.Shutdown()
		},
	}

	if panicwatchConfig != nil {
		panicwatchConfig(&config, c.logger)
	}

	if err := panicwatch.Start(config); err != nil {
		c.logger.Fatalw("failed to start panicwatch", zap.Error(err))
	}
}
