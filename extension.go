package app

import "go.uber.org/zap"

type Extension interface {
	Initialize(configLoader func(config interface{}), logger *zap.SugaredLogger) error
	Start(appCtx Context, logger *zap.SugaredLogger)
}

type appConfigExtension struct {
	config interface{}
}

func (e *appConfigExtension) Initialize(configLoader func(config interface{}), _ *zap.SugaredLogger) error {
	configLoader(e.config)

	return nil
}

func (e *appConfigExtension) Start(Context, *zap.SugaredLogger) {}
