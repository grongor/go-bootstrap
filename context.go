package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Context interface {
	context.Context

	Shutdown()
	IsShuttingDown() bool
	StartWorker(worker func())
	WaitForWorkers()
}

type CliContext struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (c *CliContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (c *CliContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *CliContext) Err() error {
	return c.ctx.Err()
}

func (c *CliContext) Value(key interface{}) interface{} {
	return nil
}

func (c *CliContext) Shutdown() {
	c.cancel()
}

func (c *CliContext) IsShuttingDown() bool {
	return c.ctx.Err() != nil
}

func (c *CliContext) StartWorker(worker func()) {
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		worker()
	}()
}

func (c *CliContext) WaitForWorkers() {
	c.wg.Wait()
}

func NewCliContext(signals ...os.Signal) *CliContext {
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM}
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalCtx, signalCancel := signal.NotifyContext(ctx, signals...)

	cliContext := &CliContext{ctx: signalCtx, cancel: cancel}

	go func() {
		<-signalCtx.Done()

		cliContext.WaitForWorkers()
		signalCancel()
	}()

	return cliContext
}
