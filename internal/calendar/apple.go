package calendar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/model"
)

// iCloudCalDAVEndpoint is Apple's public CalDAV entry point. Apple's CalDAV
// discovery flow starts here; FindCurrentUserPrincipal redirects us to a
// per-user principal URL under p01-caldav.icloud.com / p05-caldav.icloud.com
// etc. — we don't hardcode that because Apple shards users across pods.
const iCloudCalDAVEndpoint = "https://caldav.icloud.com"

// AppleSyncer has no server-side configuration: all credentials are
// per-user (Apple ID email + app-specific password, captured at connect).
type AppleSyncer struct{}

func NewAppleSyncer() *AppleSyncer { return &AppleSyncer{} }

// Validate confirms the supplied Apple ID + app-specific password can
// authenticate against iCloud CalDAV. Called from the /calendar/apple/connect
// handler so we surface "bad password" as a user-visible 401 rather than
// storing bogus credentials that only fail at reserve time.
func (AppleSyncer) Validate(ctx context.Context, email, appPassword string) error {
	if email == "" || appPassword == "" {
		return errors.New("email and app password required")
	}
	_, _, err := discoverAppleCalendars(ctx, email, appPassword)
	return err
}

// discoverAppleCalendars walks Apple's CalDAV discovery: endpoint → principal
// → calendar home → calendars. The first hop (FindCurrentUserPrincipal)
// exercises basic auth, so bad passwords fail here rather than in
// PutCalendarObject.
func discoverAppleCalendars(ctx context.Context, email, appPassword string) (calClient *caldav.Client, calendars []caldav.Calendar, err error) {
	httpClient := webdav.HTTPClientWithBasicAuth(nil, email, appPassword)

	davClient, err := webdav.NewClient(httpClient, iCloudCalDAVEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("apple webdav client: %w", err)
	}
	principal, err := davClient.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("apple principal: %w", err)
	}

	calClient, err = caldav.NewClient(httpClient, iCloudCalDAVEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("apple caldav client: %w", err)
	}
	home, err := calClient.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, nil, fmt.Errorf("apple calendar home: %w", err)
	}
	cals, err := calClient.FindCalendars(ctx, home)
	if err != nil {
		return nil, nil, fmt.Errorf("apple calendars: %w", err)
	}
	if len(cals) == 0 {
		return nil, nil, errors.New("apple account has no calendars")
	}
	return calClient, cals, nil
}

// PushEvent creates a VEVENT on the user's first iCloud calendar (typically
// "home"). We identify the event by a datil-generated UID so the object
// path is deterministic and re-runs are idempotent from the server's side.
// Apple returns no structured event id — the UID is what we stash on the
// appointments row for future update/delete in a later phase.
func (a AppleSyncer) PushEvent(ctx context.Context, ci model.CalendarIntegration, input EventInput) (string, error) {
	if ci.AccessToken == "" || ci.AccountEmail == nil {
		return "", errors.New("apple integration missing credentials")
	}
	email := *ci.AccountEmail
	appPassword := ci.AccessToken

	calClient, cals, err := discoverAppleCalendars(ctx, email, appPassword)
	if err != nil {
		return "", err
	}
	targetPath := cals[0].Path

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//datil//calendar//EN")

	uid := "datil-" + uuid.NewString() + "@datil.app"
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, uid)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	event.Props.SetDateTime(ical.PropDateTimeStart, input.Start)
	event.Props.SetDateTime(ical.PropDateTimeEnd, input.End)
	event.Props.SetText(ical.PropSummary, summaryFor(input))
	if desc := descriptionFor(input); desc != "" {
		event.Props.SetText(ical.PropDescription, desc)
	}
	cal.Children = append(cal.Children, event.Component)

	objectPath := strings.TrimSuffix(targetPath, "/") + "/" + uid + ".ics"
	if _, err := calClient.PutCalendarObject(ctx, objectPath, cal); err != nil {
		return "", fmt.Errorf("putting apple event: %w", err)
	}
	return uid, nil
}
