package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/mossandoval/datil-api/internal/config"
	"github.com/mossandoval/datil-api/internal/handler"
	"github.com/mossandoval/datil-api/internal/middleware"
)

func New(
	cfg *config.Config,
	authHandler *handler.AuthHandler,
	businessHandler *handler.BusinessHandler,
	categoryHandler *handler.CategoryHandler,
	serviceHandler *handler.ServiceHandler,
	appointmentHandler *handler.AppointmentHandler,
	scheduleHandler *handler.ScheduleHandler,
	calendarHandler *handler.CalendarHandler,
	dashboardHandler *handler.DashboardHandler,
	bookingHandler *handler.BookingHandler,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public ICS subscription feed. Mounted at the root (no /api/v1 prefix)
	// because calendar clients treat the URL as permanent — keeping it off
	// the versioned API path means a future /api/v2 cutover doesn't break
	// every owner's Apple Calendar subscription. 60 req/min per IP is
	// comfortable headroom over client poll intervals (hourly on iOS).
	r.Group(func(r chi.Router) {
		r.Use(middleware.PerIP(60, time.Minute))
		r.Get("/calendar/ics/{token}", calendarHandler.ServeFeed)
	})

	// All API routes are mounted under /api/v1 so the prefix can change in
	// future major versions without breaking existing clients.
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Route("/auth", func(r chi.Router) {
			r.Use(middleware.PerIP(5, time.Minute))
			r.Post("/signup", authHandler.Signup)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
		})

		r.Route("/book/{url}", func(r chi.Router) {
			r.Use(middleware.PerIP(20, time.Minute))
			r.Get("/", bookingHandler.GetBusiness)
			r.Get("/services", bookingHandler.GetServices)
			r.Get("/availability", bookingHandler.GetAvailability)
			// Reserve gets a tighter cap on top of the 20/min above —
			// each reserve fires a Twilio message and writes a DB row.
			r.With(middleware.PerIP(5, time.Minute)).Post("/reserve", bookingHandler.Reserve)
		})

		// OAuth callback is public: the browser lands here via Google's
		// redirect without a Bearer header. CSRF + identity come from the
		// signed `state` parameter; see calendar.StateSigner.
		r.Get("/calendar/{provider}/callback", calendarHandler.Callback)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(cfg.JWTSecret))

			r.Route("/business", func(r chi.Router) {
				r.Get("/", businessHandler.Get)
				r.Put("/", businessHandler.Update)
				r.Put("/bank", businessHandler.UpdateBank)
				r.Put("/logo", businessHandler.UpdateLogo)
			})

			r.Route("/categories", func(r chi.Router) {
				r.Get("/", categoryHandler.List)
				r.Post("/", categoryHandler.Create)
				r.Put("/{id}", categoryHandler.Update)
				r.Delete("/{id}", categoryHandler.Delete)
			})

			r.Route("/services", func(r chi.Router) {
				r.Get("/", serviceHandler.List)
				r.Post("/", serviceHandler.Create)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", serviceHandler.Get)
					r.Put("/", serviceHandler.Update)
					r.Delete("/", serviceHandler.Delete)
					r.Get("/extras", serviceHandler.ListExtras)
					r.Post("/extras", serviceHandler.LinkExtra)
					r.Delete("/extras/{extraId}", serviceHandler.UnlinkExtra)
				})
			})

			r.Route("/appointments", func(r chi.Router) {
				r.Get("/", appointmentHandler.List)
				r.Post("/", appointmentHandler.Create)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", appointmentHandler.Get)
					r.Put("/", appointmentHandler.Update)
					r.Delete("/", appointmentHandler.Delete)
					r.Put("/status", appointmentHandler.UpdateStatus)
					r.Post("/payment-proof", appointmentHandler.UpdatePaymentProof)
				})
			})

			r.Route("/schedule", func(r chi.Router) {
				r.Get("/workdays", scheduleHandler.GetWorkdays)
				r.Put("/workdays", scheduleHandler.UpdateWorkdays)
				r.Route("/personal-time", func(r chi.Router) {
					r.Get("/", scheduleHandler.ListPersonalTime)
					r.Post("/", scheduleHandler.CreatePersonalTime)
					r.Delete("/{id}", scheduleHandler.DeletePersonalTime)
				})
			})

			// ICS rotate is provider-specific (no parallel for Google), so
			// register it outside the {provider} subroute to keep the dispatch
			// explicit at the handler level.
			r.Post("/calendar/ics/rotate", calendarHandler.RotateICS)

			r.Route("/calendar/{provider}", func(r chi.Router) {
				r.Post("/connect", calendarHandler.Connect)
				r.Delete("/", calendarHandler.Disconnect)
			})

			r.Get("/dashboard", dashboardHandler.Get)
		})
	})

	return r
}
