package calendar

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/model"
)

func sampleFeed(t *testing.T) FeedInput {
	t.Helper()
	email := "cliente@example.com"
	start := time.Date(2026, 4, 30, 15, 0, 0, 0, time.UTC)
	end := start.Add(90 * time.Minute)
	return FeedInput{
		Business: model.Business{
			Name:     "Salón de María",
			Timezone: "America/Mexico_City",
		},
		Appointments: []FeedAppointment{{
			Appointment: model.Appointment{
				ID:            uuid.MustParse("6f4c7c5c-1d3a-4f8e-9a1a-3b4d5e6f7890"),
				CustomerName:  "Ana, Pérez; García",
				CustomerEmail: &email,
				CustomerPhone: "+5215512345678",
				StartTime:     start,
				EndTime:       end,
				Status:        "confirmed",
				IcalSequence:  3,
				UpdatedAt:     start.Add(-time.Hour),
			},
			ServiceNames: []string{"Corte"},
			ExtraNames:   []string{"Tinte"},
			ServiceLines: []string{"Corte — $450", "Tinte — $800"},
		}},
	}
}

func TestRenderFeed_ParseableRoundTrip(t *testing.T) {
	payload, err := RenderFeed(sampleFeed(t))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	cal, err := ical.NewDecoder(bytes.NewReader(payload)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	events := cal.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]

	uid, err := ev.Props.Text(ical.PropUID)
	if err != nil {
		t.Fatalf("uid: %v", err)
	}
	const wantUID = "6f4c7c5c-1d3a-4f8e-9a1a-3b4d5e6f7890@datil.app"
	if uid != wantUID {
		t.Fatalf("uid: got %q, want %q", uid, wantUID)
	}

	seq, err := ev.Props.Get(ical.PropSequence).Int()
	if err != nil {
		t.Fatalf("sequence: %v", err)
	}
	if seq != 3 {
		t.Fatalf("sequence: got %d, want 3", seq)
	}

	status, err := ev.Props.Text(ical.PropStatus)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "CONFIRMED" {
		t.Fatalf("status: got %q, want CONFIRMED", status)
	}
}

func TestRenderFeed_EmitsUTCWithZSuffix(t *testing.T) {
	payload, err := RenderFeed(sampleFeed(t))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(payload)
	// Quick smoke on the raw encoded form. UTC times in RFC 5545 end with Z
	// and carry no TZID parameter.
	if !strings.Contains(out, "DTSTART:20260430T150000Z") {
		t.Fatalf("expected DTSTART line with Z suffix; got:\n%s", out)
	}
	if !strings.Contains(out, "DTEND:20260430T163000Z") {
		t.Fatalf("expected DTEND line with Z suffix; got:\n%s", out)
	}
	if strings.Contains(out, "TZID=") {
		t.Fatalf("did not expect TZID parameter in UTC-only feed; got:\n%s", out)
	}
}

func TestRenderFeed_CancelledStatus(t *testing.T) {
	input := sampleFeed(t)
	input.Appointments[0].Appointment.Status = "cancelled"
	payload, err := RenderFeed(input)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	cal, err := ical.NewDecoder(bytes.NewReader(payload)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	status, err := cal.Events()[0].Props.Text(ical.PropStatus)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "CANCELLED" {
		t.Fatalf("status: got %q, want CANCELLED", status)
	}
}

func TestRenderFeed_EscapesCommasAndSemicolons(t *testing.T) {
	// Ana, Pérez; García has both chars that RFC 5545 requires escaped
	// in TEXT values. The encoder should handle them — verify the round
	// trip preserves the original string.
	input := sampleFeed(t)
	payload, err := RenderFeed(input)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	cal, err := ical.NewDecoder(bytes.NewReader(payload)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	summary, err := cal.Events()[0].Props.Text(ical.PropSummary)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if !strings.Contains(summary, "Ana, Pérez; García") {
		t.Fatalf("expected special chars to round-trip; got %q", summary)
	}
	if !strings.Contains(summary, "Corte + Tinte") {
		t.Fatalf("expected services joined with +; got %q", summary)
	}

	// Also: raw output must escape them inline so generic RFC 5545 parsers
	// don't split on a bare comma/semicolon.
	raw := string(payload)
	if !strings.Contains(raw, `Ana\, Pérez\; García`) {
		t.Fatalf("expected inline RFC 5545 escaping in raw payload; got:\n%s", raw)
	}
}

func TestRenderFeed_SequenceEncodedAsBareInteger(t *testing.T) {
	// Regression: we used to emit `SEQUENCE;VALUE=TEXT:0` because
	// Props.SetText annotates whenever the property's default type isn't
	// TEXT, and SEQUENCE defaults to INTEGER. Strict clients (Apple
	// Calendar on some macOS releases) silently drop the malformed prop,
	// which breaks reschedule-in-place detection.
	payload, err := RenderFeed(sampleFeed(t))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(payload)
	if !strings.Contains(out, "\r\nSEQUENCE:3\r\n") {
		t.Fatalf("expected bare `SEQUENCE:3` line; got:\n%s", out)
	}
	if strings.Contains(out, "SEQUENCE;VALUE=") {
		t.Fatalf("SEQUENCE must not carry a VALUE= parameter; got:\n%s", out)
	}
}

func TestRenderFeed_XWRCalnameNoValueParam(t *testing.T) {
	// Sister regression to the SEQUENCE one: X-* extensions have no
	// registered default type, so go-ical's SetText stamps VALUE=TEXT.
	// Harmless per RFC (X- defaults to TEXT anyway) but indicative of a
	// serializer gone wrong, and easy to fix at the source.
	payload, err := RenderFeed(sampleFeed(t))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(payload)
	if strings.Contains(out, "X-WR-CALNAME;VALUE=") {
		t.Fatalf("X-WR-CALNAME must not carry a VALUE= parameter; got:\n%s", out)
	}
	// And the calendar name still round-trips.
	if !strings.Contains(out, "Dátil") || !strings.Contains(out, "Salón de María") {
		t.Fatalf("expected X-WR-CALNAME to contain the business name; got:\n%s", out)
	}
}

func TestRenderFeed_UsesCRLFLineEndings(t *testing.T) {
	payload, err := RenderFeed(sampleFeed(t))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Every LF must be preceded by CR — catches accidental LF-only output
	// that breaks Apple Calendar on strict releases.
	for i := 0; i < len(payload); i++ {
		if payload[i] == '\n' {
			if i == 0 || payload[i-1] != '\r' {
				t.Fatalf("bare LF at offset %d; expected CRLF", i)
			}
		}
	}
}

func TestRenderFeed_LongLinesAreFoldedAt75Octets(t *testing.T) {
	// Build a payload whose DESCRIPTION exceeds 75 octets so we exercise
	// the RFC 5545 §3.1 line folding path. The octet count matters, not
	// the character count — multi-byte UTF-8 runes (é, —, ñ) must not be
	// split across a fold.
	input := sampleFeed(t)
	longName := "Juan Pérez García Rodríguez López Martínez Hernández Sánchez Fernández"
	input.Appointments[0].Appointment.CustomerName = longName
	input.Appointments[0].ServiceLines = []string{
		"Un servicio con un nombre bastante largo para forzar el plegado de líneas — $1000",
		"Otro servicio con varios acentos: piñata, mañana, corazón, café, lección — $2500",
	}
	payload, err := RenderFeed(input)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// No content line (between CRLFs) may exceed 75 octets. Continuation
	// lines (those starting with SP or HTAB) are the folded halves and
	// include the leading whitespace toward the budget.
	for _, line := range bytes.Split(payload, []byte("\r\n")) {
		if len(line) > 75 {
			t.Fatalf("line exceeds 75 octets (%d): %q", len(line), line)
		}
	}
	// And the folded content still round-trips through a decoder.
	cal, err := ical.NewDecoder(bytes.NewReader(payload)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	summary, err := cal.Events()[0].Props.Text(ical.PropSummary)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if !strings.Contains(summary, longName) {
		t.Fatalf("folded summary lost the long name; got %q", summary)
	}
}

func TestRenderFeed_EmptyAppointments(t *testing.T) {
	// No appointments is a valid feed — Apple Calendar subscribes happily
	// to an empty calendar.
	payload, err := RenderFeed(FeedInput{Business: model.Business{Name: "Test"}})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	cal, err := ical.NewDecoder(bytes.NewReader(payload)).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(cal.Events()) != 0 {
		t.Fatalf("expected 0 events, got %d", len(cal.Events()))
	}
}
