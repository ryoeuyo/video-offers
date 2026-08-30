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
	twitchsvc "github.com/ruslan/video-offers/internal/service/twitch"
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
	offerRepo := repo.NewOfferRepo(pool)
	twitchRepo := repo.NewTwitchLinkRepo(pool)

	var twitchChecker service.TwitchLinkChecker
	if cfg.TwitchEnabled() {
		oauthClient := twitchsvc.NewOAuthClient(
			cfg.Twitch.ClientID,
			cfg.Twitch.ClientSecret,
			cfg.Twitch.RedirectURI,
			&http.Client{Timeout: 10 * time.Second},
		)
		twitchChecker = twitchsvc.NewLinkService(
			twitchRepo,
			settingsRepo,
			userRepo,
			oauthClient,
			cfg.TwitchSealKey(),
			cfg.Twitch.FrontendSuccessURL,
			true,
		)
		log.Info("twitch integration enabled")
	} else {
		log.Info("twitch integration disabled")
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	videoResolver := video.NewCompositeResolver(video.NewYouTubeResolver(httpClient))
	var offerGates service.OfferTwitchGate
	if svc := twitchAsLinkService(twitchChecker); svc != nil {
		offerGates = svc
	}
	offerSvc := service.NewOfferService(offerRepo, userRepo, settingsRepo, videoResolver, offerGates)
	userSvc := service.NewUserService(userRepo, settingsRepo, twitchChecker)

	srv := httpapi.NewServer(cfg, log, httpapi.Deps{
		Pool:   pool,
		Auth:   authSvc,
		Users:  userSvc,
		Offers: offerSvc,
		Twitch: twitchAsLinkService(twitchChecker),
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

// twitchAsLinkService извлекает *LinkService из интерфейса для HTTP-handlers.
func twitchAsLinkService(checker service.TwitchLinkChecker) *twitchsvc.LinkService {
	if checker == nil {
		return nil
	}
	svc, ok := checker.(*twitchsvc.LinkService)
	if !ok || svc == nil {
		return nil
	}
	return svc
}
