package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/applications"
	controlauth "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/auth"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/config"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/grpcclient"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/httpapi"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/rbac"
	"github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store"
	controlstore "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := pgxpool.New(ctx, settings.DatabaseURL)
	if err != nil {
		log.Fatal("configure PostgreSQL: ", err)
	}
	defer database.Close()

	redisOptions, err := redis.ParseURL(settings.RedisURL)
	if err != nil {
		log.Fatal("configure Redis: ", err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	aiConnection, err := grpc.NewClient(
		settings.AIAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal("configure AI runtime client: ", err)
	}
	defer aiConnection.Close()

	sandboxConnection, err := grpc.NewClient(
		settings.SandboxAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal("configure sandbox client: ", err)
	}
	defer sandboxConnection.Close()

	checks := map[string]httpapi.Checker{
		"database":  httpapi.DatabaseChecker(database),
		"redis":     httpapi.RedisChecker(redisClient),
		"pgvector":  httpapi.PGVectorChecker(database),
		"aiRuntime": grpcclient.NewAIHealthChecker(aiConnection, settings.GRPCToken),
		"sandbox":   grpcclient.NewSandboxHealthChecker(sandboxConnection, settings.GRPCToken),
	}
	jwtService, err := controlauth.NewJWTService(settings.JWTSecret, settings.JWTExpiration, time.Now)
	if err != nil {
		log.Fatal("configure JWT service: ", err)
	}
	queries := controlstore.New(database)
	userRepository := store.NewUserRepository(queries)
	userService, err := controlauth.NewService(
		userRepository,
		controlauth.NewLoginLimiter(redisClient),
		jwtService,
	)
	if err != nil {
		log.Fatal("configure user service: ", err)
	}
	router := httpapi.NewRouter(httpapi.NewHealthHandler(checks, settings.HealthTimeout))
	httpapi.RegisterUserRoutes(router, httpapi.NewUserHandler(userService), jwtService)
	accessRepository := store.NewAccessRepository(queries)
	authorizer := rbac.NewAuthorizer(accessRepository)
	applicationService := applications.NewService(store.NewApplicationRepository(queries), authorizer)
	httpapi.RegisterApplicationRoutes(router, httpapi.NewApplicationHandler(applicationService), jwtService)
	server := &http.Server{
		Addr:              settings.HTTPAddress,
		Handler:           router,
		ReadHeaderTimeout: settings.HealthTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Go control plane listening on %s", settings.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), settings.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("HTTP shutdown failed: %v", err)
	}
}
