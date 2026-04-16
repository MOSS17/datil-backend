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

	// Repositories
	businessRepo := repository.NewBusinessRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	serviceRepo := repository.NewServiceRepository(pool)
	appointmentRepo := repository.NewAppointmentRepository(pool)
	scheduleRepo := repository.NewScheduleRepository(pool)
	calendarRepo := repository.NewCalendarRepository(pool)
	dashboardRepo := repository.NewDashboardRepository(pool)

	// Handlers
	authHandler := handler.NewAuthHandler(userRepo, businessRepo, pool, cfg)
	businessHandler := handler.NewBusinessHandler(businessRepo)
	categoryHandler := handler.NewCategoryHandler(categoryRepo)
	serviceHandler := handler.NewServiceHandler(serviceRepo)
	appointmentHandler := handler.NewAppointmentHandler(appointmentRepo, notifier)
	scheduleHandler := handler.NewScheduleHandler(scheduleRepo)
	calendarHandler := handler.NewCalendarHandler(calendarRepo)
	dashboardHandler := handler.NewDashboardHandler(dashboardRepo)
	bookingHandler := handler.NewBookingHandler(businessRepo, serviceRepo, appointmentRepo, scheduleRepo, notifier)

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
