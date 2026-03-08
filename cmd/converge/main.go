package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"converge-finance.com/m/internal/config"
	"converge-finance.com/m/internal/platform/database"
	httpserver "converge-finance.com/m/internal/platform/http"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: converge <command>")
		fmt.Println("Commands:")
		fmt.Println("  serve    - Start the server")
		fmt.Println("  migrate  - Run database migrations")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "serve":
		runServer()
	case "migrate":
		runMigrations()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func runServer() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	db, err := database.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	logger.Info("Connected to database")

	httpServer := httpserver.NewServer(cfg, db, logger)
	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)

	grpcServer := grpc.NewServer()
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)

	errChan := make(chan error, 2)

	go func() {
		logger.Info("Starting HTTP server", zap.String("addr", httpAddr))
		if err := httpServer.ListenAndServe(httpAddr); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	go func() {
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			errChan <- fmt.Errorf("failed to listen on gRPC port: %w", err)
			return
		}
		logger.Info("Starting gRPC server", zap.String("addr", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		logger.Error("Server error", zap.Error(err))
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	}

	logger.Info("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	grpcServer.GracefulStop()

	logger.Info("Servers shut down successfully")
}

func runMigrations() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	direction := "up"
	if len(os.Args) > 2 {
		direction = os.Args[2]
	}

	if err := database.RunMigrations(cfg.DatabaseURL, direction); err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migrations completed successfully")
}
