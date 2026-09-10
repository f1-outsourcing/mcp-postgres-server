package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/gomcpgo/mcp/pkg/handler"
	"github.com/gomcpgo/mcp/pkg/server"
	pghandler "github.com/gomcpgo/postgres/pkg/handler"
)

// TODO: hard coded prefix as the mcp-gateway-go is not sending arguments!

const version = "1.0.0"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	prefix := flag.String("prefix", "", "Prefix for tool names")
	dsn := flag.String("dsn", "", "Postgres DSN (e.g. postgresql://user:pass@host:port/db)")
	readOnly := flag.Bool("read-only", false, "Disable write tools (create/alter/insert/update/delete)")
	logLevel := flag.String("log-level", "error", "debug|info|warn|error")

	flag.Parse()

	if *showVersion {
		fmt.Println("Tool version:", version)
		os.Exit(0)
	}

	if *dsn == "" {
		*dsn = os.Getenv("PG_DSN")
	}

	if *dsn == "" {
		slog.Error("Missing DSN. Use --dsn or set PG_DSN")
		os.Exit(1)
	}

	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	default:
		level = slog.LevelError
	}

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}),
		),
	)

	// Create the postgres handler with the specified prefix
	pgHandler := pghandler.NewPostgresHandlerWithPrefix(*prefix)
	pgHandler.SetDSN(*dsn)
	pgHandler.SetReadOnly(*readOnly)

	// Create handler registry
	registry := handler.NewHandlerRegistry()
	registry.RegisterToolHandler(pgHandler)

	// Create and start server. We inject a line-based stdio transport so each
	// input line is treated as one JSON-RPC message: a single malformed line is
	// answered with a JSON-RPC error and never breaks processing of the next line.
	srv := server.New(server.Options{
		Name:      "mcp-postgres-server",
		Version:   version,
		Registry:  registry,
		Transport: newLineStdioTransport(),
	})

	slog.Info("Starting postgres server", "prefix", *prefix)
	if err := srv.Run(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
