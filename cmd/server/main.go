package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quarantine-workbench/internal/policy"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	if cfg.SelfCheck {
		return runSelfCheck(cfg)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app, err := buildApplication(ctx, cfg, policy.SystemClock{})
	if err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() { errCh <- app.serve() }()
	log.Printf("植物引种隔离放行工作台监听 %s", cfg.Addr)
	select {
	case <-ctx.Done():
		shutdownCtx, stop := context.WithTimeout(context.Background(), 8*time.Second)
		defer stop()
		return app.shutdown(shutdownCtx)
	case err = <-errCh:
		app.repo.Close()
		return err
	}
}

func combine(primary, secondary error) error {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}
	return fmt.Errorf("%v；关闭失败: %w", primary, secondary)
}
func ignoreCanceled(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
