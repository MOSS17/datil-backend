package calendar

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/mossandoval/datil-api/internal/model"
)

// FeedAppointment is the per-row input the ICS renderer needs. The caller
// (handler) precomputes services + extras into plain strings so the
// renderer stays free of repository dependencies and tests can exercise it
// without a DB.
type FeedAppointment struct {
	Appointment model.Appointment
	// ServiceNames is the flat list of services (not extras) on this
	// appointment, in a stable order. Used in SUMMARY and DESCRIPTION.
	ServiceNames []string
	// ExtraNames is the flat list of extras on this appointment.
	ExtraNames []string
	// ServiceLines is a pre-formatted line per billable line item (e.g.
	// "Corte de cabello — $450"). Used for the DESCRIPTION body.
	ServiceLines []string
}

// FeedInput carries everything the ICS renderer needs for a single
// subscription response. Business is the owner's business; appointments
// is the window the handler chose (typically now-30d .. forever).
type FeedInput struct {
	Business     model.Business
	Appointments []FeedAppointment
}

// RenderFeed emits a VCALENDAR payload. Encoding quirks in RFC 5545:
//   - Text values must escape `\`, `,`, `;`, and newlines. go-ical handles
//     that inside Props.SetText.
//   - DTSTART / DTEND are emitted as UTC (…Z suffix) so no VTIMEZONE block
//     is needed — clients render in the viewer's local zone, which for
//     the owner is just their business timezone anyway.
//   - UID must be stable forever. We use `<appointment_uuid>@datil.app` so
//     re-renders update in place via (UID, SEQUENCE) — duplicate events
//     only appear if we ever mint a new UID for the same appointment.
func RenderFeed(input FeedInput) ([]byte, error) {
	// go-ical refuses to encode a VCALENDAR with zero children (RFC 5545
	// technically requires at least one calendar component). Newly-
	// connected users have no appointments, so hand-craft the empty feed
	// to sidestep the library check.
	if len(input.Appointments) == 0 {
		return emptyFeed(input.Business.Name), nil
	}

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//datil//calendar//EN")
	cal.Props.SetText(ical.PropCalendarScale, "GREGORIAN")
	cal.Props.SetText(ical.PropMethod, "PUBLISH")
	cal.Props.SetText("X-WR-CALNAME", "Dátil — "+input.Business.Name)

	dtstamp := time.Now().UTC()
	for _, fa := range input.Appointments {
		event := ical.NewEvent()
		uid := fa.Appointment.ID.String() + "@datil.app"
		event.Props.SetText(ical.PropUID, uid)
		event.Props.SetDateTime(ical.PropDateTimeStamp, dtstamp)
		event.Props.SetDateTime(ical.PropDateTimeStart, fa.Appointment.StartTime.UTC())
		event.Props.SetDateTime(ical.PropDateTimeEnd, fa.Appointment.EndTime.UTC())
		event.Props.SetDateTime(ical.PropLastModified, fa.Appointment.UpdatedAt.UTC())
		// RFC 5545 SEQUENCE is an integer property, stringified.
		event.Props.SetText(ical.PropSequence, fmt.Sprintf("%d", fa.Appointment.IcalSequence))
		event.Props.SetText(ical.PropSummary, feedSummary(fa))
		if desc := feedDescription(fa); desc != "" {
			event.Props.SetText(ical.PropDescription, desc)
		}
		if loc := feedLocation(input.Business); loc != "" {
			event.Props.SetText(ical.PropLocation, loc)
		}
		event.Props.SetText(ical.PropStatus, feedStatus(fa.Appointment.Status))
		cal.Children = append(cal.Children, event.Component)
	}

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, fmt.Errorf("encoding ics feed: %w", err)
	}
	return buf.Bytes(), nil
}

// feedSummary: "Maria Pérez — Corte + Tinte". The joiner matches the spec
// so multi-service events are legible at a glance in the mobile calendar
// mini-cell. Single-service fallbacks just show the service name.
func feedSummary(fa FeedAppointment) string {
	parts := []string{}
	if fa.Appointment.CustomerName != "" {
		parts = append(parts, fa.Appointment.CustomerName)
	}
	items := append([]string(nil), fa.ServiceNames...)
	items = append(items, fa.ExtraNames...)
	if joined := strings.Join(items, " + "); joined != "" {
		parts = append(parts, joined)
	}
	return strings.Join(parts, " — ")
}

func feedDescription(fa FeedAppointment) string {
	lines := []string{}
	if fa.Appointment.CustomerPhone != "" {
		lines = append(lines, "Teléfono: "+fa.Appointment.CustomerPhone)
	}
	if fa.Appointment.CustomerEmail != nil && *fa.Appointment.CustomerEmail != "" {
		lines = append(lines, "Email: "+*fa.Appointment.CustomerEmail)
	}
	if len(fa.ServiceLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Servicios:")
		for _, line := range fa.ServiceLines {
			lines = append(lines, "  "+line)
		}
	}
	if fa.Appointment.AdvancePaymentImageURL != nil && *fa.Appointment.AdvancePaymentImageURL != "" {
		lines = append(lines, "")
		lines = append(lines, "Anticipo registrado")
	}
	return strings.Join(lines, "\n")
}

func feedLocation(b model.Business) string {
	if b.Location == nil {
		return ""
	}
	return strings.TrimSpace(*b.Location)
}

// emptyFeed renders a valid-but-empty VCALENDAR. escapeText applies the
// subset of RFC 5545 text escaping we need for the one free-form value
// (business name in X-WR-CALNAME).
func emptyFeed(businessName string) []byte {
	return []byte("BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//datil//calendar//EN\r\n" +
		"CALSCALE:GREGORIAN\r\n" +
		"METHOD:PUBLISH\r\n" +
		"X-WR-CALNAME:Dátil — " + escapeText(businessName) + "\r\n" +
		"END:VCALENDAR\r\n")
}

func escapeText(s string) string {
	// RFC 5545 §3.3.11: backslash first (so we don't double-escape the
	// escapes we're about to add), then newline, comma, semicolon.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, ";", `\;`)
	return s
}

// feedStatus maps datil's appointment status vocabulary onto RFC 5545's.
// Unknown values fall back to CONFIRMED so we never emit an invalid feed.
func feedStatus(s string) string {
	switch s {
	case "cancelled":
		return "CANCELLED"
	case "completed", "confirmed", "pending":
		return "CONFIRMED"
	default:
		return "CONFIRMED"
	}
}
