package main

import (
	"context"
	"flag"
	"fmt"
	"gostarter/internal/app"
	"gostarter/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	envFileFlag := flag.String("env", "", "Path to custom .env configuration file (optional)")
	portFlag := flag.String("port", "", "HTTP server port override (optional, defaults to config)")
	dbDSNFlag := flag.String("db", "", "PostgreSQL connection string override (optional, defaults to config)")
	migrateOnly := flag.Bool("migrate-only", false, "apply database migrations and exit")
	flag.Parse()

	// Load configuration from .env and environment variables via Viper
	cfg, err := config.Load(*envFileFlag)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// Apply CLI flag overrides if explicitly specified
	if *portFlag != "" {
		cfg.App.Port = *portFlag
	}
	if *dbDSNFlag != "" {
		cfg.Database.DSN = *dbDSNFlag
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return app.Run(ctx, cfg, app.Options{MigrateOnly: *migrateOnly})
}
