package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
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

	slaTimeout := time.Duration(cfg.SLA.MaxResponseTimeMs) * time.Millisecond
	chatSummaryHandler := handler.NewChatSummaryHandler(
		userService,
		vectorService,
		permissionsService,
		slaTimeout,
	)

	router := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/api/v1/chat/summary":
			chatSummaryHandler.Handle(ctx)
		case "/health":
			healthCheckHandler(ctx)
		default:
			ctx.SetStatusCode(fasthttp.StatusNotFound)
		}
	}

	server := &fasthttp.Server{
		Handler:      router,
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

		_ = server.ShutdownWithContext(shutdownCtx)
	}()

	addr := fmt.Sprintf(":%s", cfg.App.Port)
	log.Printf("%s starting on %s (SLA: %dms)", cfg.App.ServiceName, addr, cfg.SLA.MaxResponseTimeMs)

	if err := server.ListenAndServe(addr); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}

	log.Println("Server stopped")
}

func healthCheckHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprintf(ctx, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}
