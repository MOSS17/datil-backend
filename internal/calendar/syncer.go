// Package calendar implements push-only integrations between datil
// appointments and external calendar providers (Google Calendar via OAuth
// 2.0 and iCloud Calendar via CalDAV). Phase 6 scope is push-only: a new
// appointment emits an event to every active integration the owner has.
// Pull sync (subtracting external events from availability) is a deferred
// phase; this package carries no read-side logic.
package calendar

import (
	"context"
	"strings"
	"time"

	"github.com/mossandoval/datil-api/internal/model"
)

// EventInput carries the minimum facts each provider needs to build an
// external calendar event from a datil appointment. Business timezone is
// an IANA name (e.g. "America/Mexico_City") — Google stores the datetime +
// tz so the owner sees the local wall-clock regardless of their device;
// Apple VEVENTs carry the zone implicitly in the time.Time.
type EventInput struct {
	BusinessName  string
	CustomerName  string
	CustomerPhone string
	CustomerEmail string
	Services      []string
	Start         time.Time
	End           time.Time
	Timezone      string
}

// Syncer pushes an appointment to a single provider. Implementations must
// be safe to call from a goroutine: no shared mutable state, timeouts
// honored, errors wrap the underlying cause so logs are useful.
type Syncer interface {
	PushEvent(ctx context.Context, ci model.CalendarIntegration, input EventInput) (externalID string, err error)
}

// NoopSyncer is the degrade-gracefully default: when a provider isn't
// configured (Google creds missing) the dispatcher falls back here so the
// booking flow keeps working and the tests don't need real credentials.
type NoopSyncer struct{}

func (NoopSyncer) PushEvent(context.Context, model.CalendarIntegration, EventInput) (string, error) {
	return "", nil
}

// DispatchingSyncer is what the booking handler holds: a single Syncer that
// routes by integration.Provider. An unknown provider is a silent skip so
// adding a new one (e.g. "outlook") doesn't retroactively error on rows
// written before this code was deployed.
type DispatchingSyncer struct {
	Google Syncer
	Apple  Syncer
}

func (d DispatchingSyncer) PushEvent(ctx context.Context, ci model.CalendarIntegration, input EventInput) (string, error) {
	switch ci.Provider {
	case "google":
		if d.Google == nil {
			return "", nil
		}
		return d.Google.PushEvent(ctx, ci, input)
	case "apple":
		if d.Apple == nil {
			return "", nil
		}
		return d.Apple.PushEvent(ctx, ci, input)
	default:
		return "", nil
	}
}

// summaryFor is the event title. Keep it short — calendar mobile views
// truncate aggressively.
func summaryFor(input EventInput) string {
	services := joinServices(input.Services)
	if services == "" {
		return input.CustomerName
	}
	return input.CustomerName + " — " + services
}

// descriptionFor is the event body. Deterministic order so edits diff cleanly.
func descriptionFor(input EventInput) string {
	parts := []string{}
	if services := joinServices(input.Services); services != "" {
		parts = append(parts, "Servicios: "+services)
	}
	if input.CustomerPhone != "" {
		parts = append(parts, "Teléfono: "+input.CustomerPhone)
	}
	if input.CustomerEmail != "" {
		parts = append(parts, "Email: "+input.CustomerEmail)
	}
	return strings.Join(parts, "\n")
}

func joinServices(s []string) string {
	return strings.Join(s, ", ")
}
