package main

import (
	"context"
	"strings"
	"time"

	app "github.com/grongor/go-bootstrap"
	"go.uber.org/zap"
)

func main() {
	var config struct{}
	app.WithConfig(&config).Start(func(appCtx app.Context, logger *zap.SugaredLogger) {
		go func() {
			time.Sleep(time.Second * 2)
			appCtx.Shutdown()
		}()
		appCtx.StartWorker(func() {
			println("running")

			for {
				println(strings.Repeat("wow", 100))

				select {
				case <-appCtx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		})
	})
	return
	app.WithConfig(&config).Run(func(appCtx context.Context, logger *zap.SugaredLogger) {
		println("running")

		for {
			println(strings.Repeat("wow", 10000000))

			select {
			case <-appCtx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	})
}
