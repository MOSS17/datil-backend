package calendar

import (
	"bytes"
	"fmt"
	"strconv"
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
	// X-WR-CALNAME isn't in go-ical's default-type table, so Props.SetText
	// would stamp a spurious `VALUE=TEXT` parameter onto it. Build the
	// prop directly — and escape the free-form business name ourselves —
	// so the output is the canonical "X-WR-CALNAME:..." form.
	setRawProp(cal.Props, "X-WR-CALNAME", escapeText("Dátil — "+input.Business.Name))

	dtstamp := time.Now().UTC()
	for _, fa := range input.Appointments {
		event := ical.NewEvent()
		uid := fa.Appointment.ID.String() + "@datil.app"
		event.Props.SetText(ical.PropUID, uid)
		event.Props.SetDateTime(ical.PropDateTimeStamp, dtstamp)
		event.Props.SetDateTime(ical.PropDateTimeStart, fa.Appointment.StartTime.UTC())
		event.Props.SetDateTime(ical.PropDateTimeEnd, fa.Appointment.EndTime.UTC())
		event.Props.SetDateTime(ical.PropLastModified, fa.Appointment.UpdatedAt.UTC())
		// SEQUENCE is INTEGER-typed (RFC 5545 §3.8.7.4); go-ical's
		// Props.SetText would emit `SEQUENCE;VALUE=TEXT:0`, which strict
		// clients (Apple Calendar on some macOS releases) drop silently —
		// breaking reschedule-in-place. Build the prop directly so the
		// line encodes as bare `SEQUENCE:0`.
		setRawProp(event.Props, ical.PropSequence, strconv.Itoa(fa.Appointment.IcalSequence))
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
	return foldLines(buf.Bytes()), nil
}

// setRawProp installs a property with a literal value and an empty Params
// map. go-ical's higher-level setters (Props.SetText, Prop.SetDateTime)
// call SetValueType internally, which stamps `VALUE=TEXT` (or other) onto
// the output whenever the supplied type doesn't match the property's
// registered default. RFC 5545 only requires `VALUE=` when *overriding* a
// known default to a different type — not when emitting the default type
// itself — so that behavior produces technically-valid-but-ugly output
// for TEXT-defaulted props, and *malformed* output for non-TEXT props
// (SEQUENCE being the concrete breakage this works around). Callers are
// responsible for applying RFC 5545 §3.3.11 text escaping where needed.
func setRawProp(props ical.Props, name, value string) {
	props.Set(&ical.Prop{
		Name:   name,
		Value:  value,
		Params: make(ical.Params),
	})
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

// emptyFeed renders a valid-but-empty VCALENDAR. go-ical's encoder
// refuses to serialize a calendar with zero children, so we hand-roll the
// payload here. escapeText applies RFC 5545 §3.3.11 escaping to the one
// free-form value (business name in X-WR-CALNAME); foldLines applies the
// §3.1 75-octet line folding in case the business name is very long.
func emptyFeed(businessName string) []byte {
	raw := []byte("BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//datil//calendar//EN\r\n" +
		"CALSCALE:GREGORIAN\r\n" +
		"METHOD:PUBLISH\r\n" +
		"X-WR-CALNAME:Dátil — " + escapeText(businessName) + "\r\n" +
		"END:VCALENDAR\r\n")
	return foldLines(raw)
}

// foldLines applies RFC 5545 §3.1 line folding to a CRLF-delimited stream.
// Content lines longer than 75 octets are split by inserting CRLF followed
// by a single SP; subsequent segments reserve one octet for that SP. Cuts
// respect UTF-8 boundaries — important for a Spanish-language UI whose
// business names, customer names, and service names routinely contain
// multi-byte runes (á é í ó ú ñ, em-dash, etc.). go-ical's encoder emits
// raw unfolded lines; Apple Calendar tolerates that in some versions and
// rejects in others, so we normalize here.
func foldLines(raw []byte) []byte {
	const maxOctets = 75
	var out bytes.Buffer
	for len(raw) > 0 {
		lineEnd := bytes.Index(raw, []byte("\r\n"))
		var line []byte
		var terminated bool
		if lineEnd < 0 {
			line, raw = raw, nil
		} else {
			line = raw[:lineEnd]
			raw = raw[lineEnd+2:]
			terminated = true
		}
		foldSingle(&out, line, maxOctets)
		if terminated {
			out.WriteString("\r\n")
		}
	}
	return out.Bytes()
}

func foldSingle(out *bytes.Buffer, line []byte, maxOctets int) {
	if len(line) <= maxOctets {
		out.Write(line)
		return
	}
	cut := utf8SafeCut(line, maxOctets)
	if cut == 0 {
		// Degenerate: a single multi-byte rune wider than the budget.
		// Force forward progress by emitting the whole rune.
		cut = runeStartAdvance(line)
	}
	out.Write(line[:cut])
	rest := line[cut:]
	for len(rest) > 0 {
		// Continuation lines include a leading SP that counts toward the
		// 75-octet limit, so the payload-per-line is maxOctets-1.
		out.WriteString("\r\n ")
		cut := utf8SafeCut(rest, maxOctets-1)
		if cut == 0 {
			cut = runeStartAdvance(rest)
		}
		out.Write(rest[:cut])
		rest = rest[cut:]
	}
}

// utf8SafeCut returns the largest index ≤ maxOctets that lands on a UTF-8
// start byte (or ASCII), so a rune is never split across a fold.
func utf8SafeCut(b []byte, maxOctets int) int {
	if len(b) <= maxOctets {
		return len(b)
	}
	cut := maxOctets
	for cut > 0 && (b[cut]&0xC0) == 0x80 {
		cut--
	}
	return cut
}

// runeStartAdvance returns the length of the leading rune in b. Used only
// in the degenerate case where a single rune is larger than the fold
// budget — emitting it whole keeps the line slightly over 75 octets but
// avoids producing invalid UTF-8.
func runeStartAdvance(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	// Detect start byte and compute rune length from its high bits.
	switch {
	case b[0]&0x80 == 0x00:
		return 1
	case b[0]&0xE0 == 0xC0:
		return 2
	case b[0]&0xF0 == 0xE0:
		return 3
	case b[0]&0xF8 == 0xF0:
		return 4
	default:
		return 1
	}
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
