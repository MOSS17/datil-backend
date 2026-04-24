package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mossandoval/datil-api/internal/config"
	"github.com/mossandoval/datil-api/internal/handler"
	"github.com/mossandoval/datil-api/internal/notification"
	"github.com/mossandoval/datil-api/internal/repository"
	"github.com/mossandoval/datil-api/internal/router"
	"github.com/mossandoval/datil-api/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()

	pool, err := config.NewDatabasePool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	// Notifier
	var notifier notification.Notifier
	if cfg.TwilioAccountSID != "" {
		notifier = notification.NewTwilioNotifier(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioWhatsAppFrom)
	} else {
		notifier = &notification.NoopNotifier{}
	}

	// Storage uploader
	uploader, err := newUploader(cfg)
	if err != nil {
		log.Fatalf("initializing uploader: %v", err)
	}

	// Repositories
	businessRepo := repository.NewBusinessRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	refreshRepo := repository.NewRefreshTokenRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	appointmentRepo := repository.NewAppointmentRepository(pool)
	scheduleRepo := repository.NewScheduleRepository(pool)
	calendarRepo := repository.NewCalendarRepository(pool)
	dashboardRepo := repository.NewDashboardRepository(pool, appointmentRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(userRepo, businessRepo, refreshRepo, pool, cfg)
	businessHandler := handler.NewBusinessHandler(businessRepo, uploader)
	categoryHandler := handler.NewCategoryHandler(categoryRepo)
	serviceHandler := handler.NewServiceHandler(serviceRepo)
	appointmentHandler := handler.NewAppointmentHandler(appointmentRepo, businessRepo, serviceRepo, uploader, pool)
	scheduleHandler := handler.NewScheduleHandler(scheduleRepo)
	calendarHandler := handler.NewCalendarHandler(calendarRepo)
	dashboardHandler := handler.NewDashboardHandler(dashboardRepo, businessRepo)
	bookingHandler := handler.NewBookingHandler(
		businessRepo, userRepo, categoryRepo, serviceRepo,
		appointmentRepo, scheduleRepo, uploader, notifier, pool,
	)

	// Router
	r := router.New(
		cfg,
		authHandler,
		businessHandler,
		categoryHandler,
		serviceHandler,
		appointmentHandler,
		scheduleHandler,
		calendarHandler,
		dashboardHandler,
		bookingHandler,
	)

	// In dev, serve the local upload directory so logos uploaded via the
	// LocalDiskUploader are reachable at LOCAL_PUBLIC_BASE_URL. Skipped in
	// production where the uploader writes to R2.
	if cfg.Env == "development" && cfg.StorageProvider == "local" {
		mux := http.NewServeMux()
		mux.Handle("/static/uploads/", http.StripPrefix("/static/uploads/", http.FileServer(http.Dir(cfg.LocalUploadRoot))))
		mux.Handle("/", r)
		r = mux
	}

	// Server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	log.Println("server stopped")
}

func newUploader(cfg *config.Config) (storage.Uploader, error) {
	switch cfg.StorageProvider {
	case "r2":
		return storage.NewR2Uploader(storage.R2Config{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			Bucket:          cfg.R2Bucket,
			PublicBaseURL:   cfg.R2PublicBaseURL,
		})
	default:
		return storage.NewLocalDiskUploader(cfg.LocalUploadRoot, cfg.LocalPublicBaseURL)
	}
}
