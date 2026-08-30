package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/pkg/jwt"
	"github.com/ruslan/video-offers/internal/pkg/logger"
	"github.com/ruslan/video-offers/internal/repo"
	"github.com/ruslan/video-offers/internal/service"
	"github.com/ruslan/video-offers/internal/service/video"
	httpapi "github.com/ruslan/video-offers/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// .env — удобство для dev, в проде переменные приходят из окружения.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(log)

	// Контекст живёт до SIGINT/SIGTERM и гасит всё, что к нему привязано.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := repo.NewPool(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()
	log.Info("database connected", "max_conns", cfg.DB.MaxConns)

	userRepo := repo.NewUserRepo(pool)
	refreshRepo := repo.NewRefreshTokenRepo(pool)
	settingsRepo := repo.NewStreamerSettingsRepo(pool)
	jwtSvc := jwt.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL)
	authSvc := service.NewAuthService(userRepo, refreshRepo, jwtSvc, cfg.Auth.RefreshTTL)
	userSvc := service.NewUserService(userRepo, settingsRepo)
	offerRepo := repo.NewOfferRepo(pool)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	videoResolver := video.NewCompositeResolver(video.NewYouTubeResolver(httpClient))
	offerSvc := service.NewOfferService(offerRepo, userRepo, settingsRepo, videoResolver)

	srv := httpapi.NewServer(cfg, log, httpapi.Deps{
		Pool:   pool,
		Auth:   authSvc,
		Users:  userSvc,
		Offers: offerSvc,
		JWT:    jwtSvc,
	})

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received", "timeout", cfg.ShutdownTimeout)
	}

	// Свой контекст: родительский уже отменён сигналом.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("graceful shutdown timed out after %s", cfg.ShutdownTimeout)
		}
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Info("stopped cleanly")
	return nil
}
