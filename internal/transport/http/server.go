package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ruslan/video-offers/internal/config"
	"github.com/ruslan/video-offers/internal/domain"
	jwtpkg "github.com/ruslan/video-offers/internal/pkg/jwt"
	"github.com/ruslan/video-offers/internal/service"
)

type Server struct {
	app *fiber.App
	cfg config.Config
	log *slog.Logger
}

// Deps — всё, от чего зависит транспорт.
type Deps struct {
	Pool   *pgxpool.Pool
	Auth   *service.AuthService
	Users  *service.UserService
	Offers *service.OfferService
	JWT    *jwtpkg.Service
}

func NewServer(cfg config.Config, log *slog.Logger, deps Deps) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "video-offers",
		ErrorHandler:          errorHandler(log),
		DisableStartupMessage: true,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          15 * time.Second,
		IdleTimeout:           60 * time.Second,
		BodyLimit:             1 << 20, // 1 MiB: сюда ходят только JSON-запросы
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(requestLogger(log))

	s := &Server{app: app, cfg: cfg, log: log}
	s.routes(deps)
	return s
}

func (s *Server) routes(deps Deps) {
	s.app.Get("/healthz", healthHandler(deps.Pool))

	v1 := s.app.Group("/api/v1")

	authH := NewAuthHandlers(deps.Auth)
	meH := NewMeHandlers(deps.Users)
	streamerH := NewStreamerHandlers(deps.Users)
	offerH := NewOfferHandlers(deps.Offers)
	requireAuth := RequireAuth(deps.JWT)
	requireStreamer := RequireStreamer()

	auth := v1.Group("/auth")
	auth.Post("/register", authH.Register)
	auth.Post("/login", authH.Login)
	auth.Post("/refresh", authH.Refresh)
	auth.Post("/logout", requireAuth, authH.Logout)

	streamers := v1.Group("/streamers")
	streamers.Get("/", streamerH.List)
	streamers.Get("/:username", streamerH.GetByUsername)
	streamers.Post("/:username/offers", requireAuth, offerH.Create)

	me := v1.Group("/me", requireAuth)
	me.Get("/", authH.Me)
	me.Patch("/", meH.Update)
	me.Get("/settings", requireStreamer, meH.GetSettings)
	me.Patch("/settings", requireStreamer, meH.UpdateSettings)
	me.Get("/offers", requireStreamer, offerH.ListQueue)
	me.Patch("/offers/:id", requireStreamer, offerH.UpdateStatus)
	me.Delete("/offers/:id", requireStreamer, offerH.Delete)
	me.Get("/sent", offerH.ListSent)
	me.Delete("/sent/:id", offerH.RevokeSent)
}

func (s *Server) Start() error {
	s.log.Info("http server listening", "addr", s.cfg.Addr, "env", s.cfg.Env)
	return s.app.Listen(s.cfg.Addr)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}

// App нужен тестам: запросы гоняются через app.Test без реального сокета.
func (s *Server) App() *fiber.App { return s.app }

func requestLogger(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		// Ошибку в ответ пишет ErrorHandler, но он отрабатывает уже после всех
		// middleware, поэтому финальный статус считаем сами.
		status := c.Response().StatusCode()
		if err != nil {
			status = statusForError(err)
		}

		level := slog.LevelInfo
		if status >= fiber.StatusInternalServerError {
			level = slog.LevelError
		}
		log.Log(c.UserContext(), level, "request",
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.Locals(requestid.ConfigDefault.ContextKey),
		)
		return err
	}
}

func statusForError(err error) int {
	if domErr, ok := domain.AsError(err); ok {
		return statusFor(domErr.Kind)
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return fiberErr.Code
	}
	return fiber.StatusInternalServerError
}
