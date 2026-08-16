package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"ejina-microgrid/internal/app"
	"ejina-microgrid/internal/config"
	"ejina-microgrid/internal/scheduler"
	"ejina-microgrid/internal/store"
	httptransport "ejina-microgrid/internal/transport/http"
)

func main() {
	cfgPath := os.Getenv("GRID_CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.json"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	st := store.New()
	svc := app.NewService(st, app.RealClock(), cfg, app.NewAtomicIDGenerator())
	svc.SeedDefaultLoads()

	sch := scheduler.New(svc, cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	sch.Start(ctx)

	handler := httptransport.NewHandler(svc)
	server := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.ServerPort),
		Handler:      handler.Routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		log.Printf("microgrid dispatch service listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}
