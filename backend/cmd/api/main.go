package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devsixt/careerpad/backend/internal/config"
	"github.com/devsixt/careerpad/backend/internal/platform/auth"
	"github.com/devsixt/careerpad/backend/internal/platform/jobsite"
	"github.com/devsixt/careerpad/backend/internal/platform/notify"
	"github.com/devsixt/careerpad/backend/internal/platform/ratelimit"
	"github.com/devsixt/careerpad/backend/internal/repository/postgres"
	transport "github.com/devsixt/careerpad/backend/internal/transport/http"
	"github.com/devsixt/careerpad/backend/internal/usecase"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL, postgres.Options{
		MaxConns:  cfg.DBMaxConns,
		QueryMode: cfg.DBQueryMode,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	// 의존성 조립: repository → usecase → transport.
	var (
		profiles = postgres.NewProfileRepository(pool)
		projects = postgres.NewProjectRepository(pool)
		searches = postgres.NewJobSearchRepository(pool)
		contacts = postgres.NewContactRepository(pool)

		mailer = notify.NewSMTPMailer(notify.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.MailFrom,
		})
		sms = notify.NewSolapiSender(notify.SolapiConfig{
			APIKey: cfg.SolapiAPIKey, APISecret: cfg.SolapiAPISecret, From: cfg.SMSFrom,
		})
		sources = jobsite.Sources(jobsite.Options{
			Client:           &http.Client{Timeout: 10 * time.Second},
			SaraminAccessKey: cfg.SaraminAccessKey,
		})
	)

	router := transport.NewRouter(transport.Deps{
		Config:   cfg,
		Verifier: auth.NewVerifier(cfg.SupabaseURL),
		Profile:  usecase.NewProfile(profiles, projects),
		Project:  usecase.NewProject(projects),
		Job:      usecase.NewJob(searches, sources),
		Contact: usecase.NewContact(profiles, contacts, mailer, sms,
			ratelimit.New(cfg.ContactPerHour, time.Hour),
			usecase.ContactOptions{SMSDailyLimit: cfg.SMSDailyLimit, PublicURL: cfg.PublicURL}),
	})

	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// job++ 결과는 여러 사이트를 묻느라 오래 걸릴 수 있다 (사이트당 12초 제한)
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", server.Addr, "mail", mailer.Enabled(), "sms", sms.Enabled())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
