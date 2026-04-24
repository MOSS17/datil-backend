package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mossandoval/datil-api/internal/model"
	"github.com/mossandoval/datil-api/internal/repository"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	calendarapi "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// googleScopes grants exactly what push-sync needs: write events on the
// user's calendars + enough OIDC to discover their email for display.
// Avoid broader scopes so Google's OAuth verification scope review (if we
// ever submit for production consent-screen approval) stays light.
var googleScopes = []string{
	calendarapi.CalendarEventsScope,
	"openid",
	"email",
}

// GoogleSyncer wraps the OAuth config + the repo so token rotation
// (which happens transparently inside oauth2.TokenSource.Token()) can be
// persisted — otherwise the refreshed access token stays in memory and we
// pay a new refresh round-trip on every reserve.
type GoogleSyncer struct {
	oauth *oauth2.Config
	repo  repository.CalendarRepository
}

// NewGoogleSyncer returns nil if any of the three creds are missing so the
// caller can fall back to NoopSyncer. Validation at config-load time
// enforces all-or-nothing, so a nil here means the deployment genuinely
// hasn't opted in to Google Calendar.
func NewGoogleSyncer(clientID, clientSecret, redirectURL string, repo repository.CalendarRepository) *GoogleSyncer {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}
	return &GoogleSyncer{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     googleoauth.Endpoint,
			Scopes:       googleScopes,
		},
		repo: repo,
	}
}

// AuthCodeURL builds the consent URL. access_type=offline + prompt=consent
// is deliberate: Google only issues a refresh token the first time a user
// grants consent. Without prompt=consent a re-connecting user silently
// skips consent and we don't get a refresh token back — which bricks
// future pushes when the access token expires.
func (g *GoogleSyncer) AuthCodeURL(state string) string {
	return g.oauth.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// Exchange swaps the authorization code for tokens. It also fetches the
// user's email via the OIDC userinfo endpoint so the integration row can
// display which Google account is connected. Failure to read userinfo is
// non-fatal: the tokens still work even if the email stays blank.
func (g *GoogleSyncer) Exchange(ctx context.Context, code string) (*oauth2.Token, string, error) {
	tok, err := g.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("oauth exchange: %w", err)
	}
	email := ""
	client := g.oauth.Client(ctx, tok)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			var body struct {
				Email string `json:"email"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			email = body.Email
		}
	}
	return tok, email, nil
}

// PushEvent is the hot path: called from the booking Reserve post-commit
// goroutine. It refreshes the access token if needed, persists any
// rotation, then inserts an event on the user's primary calendar.
func (g *GoogleSyncer) PushEvent(ctx context.Context, ci model.CalendarIntegration, input EventInput) (string, error) {
	if ci.RefreshToken == nil || *ci.RefreshToken == "" {
		return "", errors.New("google integration missing refresh token")
	}
	if ci.AccessToken == nil {
		return "", errors.New("google integration missing access token")
	}
	initial := &oauth2.Token{
		AccessToken:  *ci.AccessToken,
		RefreshToken: *ci.RefreshToken,
	}
	if ci.ExpiresAt != nil {
		initial.Expiry = *ci.ExpiresAt
	}

	ts := g.oauth.TokenSource(ctx, initial)
	current, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("refreshing google token: %w", err)
	}

	// Persist rotated tokens. Best-effort: if the Upsert fails we still push
	// successfully this round; next push just does another refresh.
	if current.AccessToken != initial.AccessToken {
		rotated := ci
		at := current.AccessToken
		rotated.AccessToken = &at
		expiry := current.Expiry
		rotated.ExpiresAt = &expiry
		if current.RefreshToken != "" {
			rt := current.RefreshToken
			rotated.RefreshToken = &rt
		}
		_ = g.repo.Upsert(ctx, &rotated)
	}

	svc, err := calendarapi.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(current)))
	if err != nil {
		return "", fmt.Errorf("building calendar service: %w", err)
	}

	tz := input.Timezone
	if tz == "" {
		tz = "UTC"
	}
	event := &calendarapi.Event{
		Summary:     summaryFor(input),
		Description: descriptionFor(input),
		Start: &calendarapi.EventDateTime{
			DateTime: input.Start.Format(time.RFC3339),
			TimeZone: tz,
		},
		End: &calendarapi.EventDateTime{
			DateTime: input.End.Format(time.RFC3339),
			TimeZone: tz,
		},
	}
	created, err := svc.Events.Insert("primary", event).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("inserting google event: %w", err)
	}
	return created.Id, nil
}
