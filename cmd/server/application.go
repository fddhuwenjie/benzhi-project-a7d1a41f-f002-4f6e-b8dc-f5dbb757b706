package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"quarantine-workbench/internal/policy"
	"quarantine-workbench/internal/repository"
	webapp "quarantine-workbench/internal/web"
	"quarantine-workbench/internal/workflow"
)

type application struct {
	repo     *repository.Repository
	service  *workflow.Service
	server   *http.Server
	listener net.Listener
}

func buildApplication(ctx context.Context, cfg config, clock policy.Clock) (*application, error) {
	repo, err := repository.Open(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	service := workflow.New(repo, clock)
	web := webapp.New(service)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		repo.Close()
		return nil, fmt.Errorf("监听 %s: %w", cfg.Addr, err)
	}
	server := &http.Server{Handler: web.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	return &application{repo: repo, service: service, server: server, listener: listener}, nil
}

func (a *application) serve() error {
	err := a.server.Serve(a.listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (a *application) shutdown(ctx context.Context) error {
	err := a.server.Shutdown(ctx)
	closeErr := a.repo.Close()
	if err != nil {
		return err
	}
	return closeErr
}
