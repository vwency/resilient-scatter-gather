package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vwency/resilient-scatter-gather/internal/handler"
	"github.com/vwency/resilient-scatter-gather/internal/services"
	"github.com/vwency/resilient-scatter-gather/pkg/config"
	pb_permissions "github.com/vwency/resilient-scatter-gather/proto/permissions"
	pb_user "github.com/vwency/resilient-scatter-gather/proto/user"
	pb_vector "github.com/vwency/resilient-scatter-gather/proto/vector"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var cfg config.ServiceConfig
	config.Init(os.Getenv("APP_ENV"), "api_gateway", &cfg)

	ctx := context.Background()

	userConn, err := grpc.NewClient(
		cfg.Grpc.UserService,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to UserService: %v", err)
	}
	defer userConn.Close()

	vectorConn, err := grpc.NewClient(
		cfg.Grpc.VectorService,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to VectorMemoryService: %v", err)
	}
	defer vectorConn.Close()

	permissionsConn, err := grpc.NewClient(
		cfg.Grpc.PermissionsService,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to PermissionsService: %v", err)
	}
	defer permissionsConn.Close()

	userService := services.NewUserServiceClient(
		pb_user.NewUserServiceClient(userConn),
		cfg.GetUserDegradationTimeout(),
	)

	vectorService := services.NewVectorMemoryServiceClient(
		pb_vector.NewVectorMemoryServiceClient(vectorConn),
		cfg.GetVectorDegradationTimeout(),
	)

	permissionsService := services.NewPermissionsServiceClient(
		pb_permissions.NewPermissionsServiceClient(permissionsConn),
		cfg.GetPermissionsDegradationTimeout(),
	)

	slaTimeout := time.Duration(cfg.TTL.MaxResponseTimeMs) * time.Millisecond
	chatSummaryHandler := handler.NewChatSummaryHandler(
		userService,
		vectorService,
		permissionsService,
		slaTimeout,
	)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/chat/summary", chatSummaryHandler)
	mux.HandleFunc("/health", healthCheckHandler)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	addr := fmt.Sprintf(":%s", cfg.App.Port)
	log.Printf("%s starting on %s (SLA: %dms)", cfg.App.ServiceName, addr, cfg.TTL.MaxResponseTimeMs)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}

	log.Println("Server stopped")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}
