package app

import (
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type Extension interface {
	GlobalConfigDecodeHooks() []mapstructure.DecodeHookFunc
	Initialize(
		configLoader func(config interface{}, decodeHooks ...mapstructure.DecodeHookFunc),
		logger *zap.SugaredLogger,
	) error
	Start(appCtx Context, logger *zap.SugaredLogger)
}

type appConfigExtension struct {
	config      interface{}
	decodeHooks []mapstructure.DecodeHookFunc
}

func (e *appConfigExtension) GlobalConfigDecodeHooks() []mapstructure.DecodeHookFunc {
	return e.decodeHooks
}

func (e *appConfigExtension) Initialize(
	configLoader func(config interface{}, decodeHooks ...mapstructure.DecodeHookFunc),
	_ *zap.SugaredLogger,
) error {
	configLoader(e.config)

	return nil
}

func (*appConfigExtension) Start(Context, *zap.SugaredLogger) {}
