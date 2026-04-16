package notification

import (
	"context"
	"fmt"
	"strings"

	"github.com/mossandoval/datil-api/internal/model"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"

	twilio "github.com/twilio/twilio-go"
)

type TwilioNotifier struct {
	client *twilio.RestClient
	from   string
}

func NewTwilioNotifier(accountSID, authToken, from string) *TwilioNotifier {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSID,
		Password: authToken,
	})
	return &TwilioNotifier{
		client: client,
		from:   from,
	}
}

func (n *TwilioNotifier) SendBookingConfirmation(ctx context.Context, phone string, details model.BookingDetails) error {
	body := fmt.Sprintf(
		"Hola %s, tu cita en %s ha sido confirmada para %s. Servicios: %s",
		details.CustomerName,
		details.BusinessName,
		details.StartTime.Format("02/01/2006 15:04"),
		strings.Join(details.Services, ", "),
	)

	to := "whatsapp:" + phone
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(to)
	params.SetFrom(n.from)
	params.SetBody(body)

	_, err := n.client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("sending WhatsApp message: %w", err)
	}

	return nil
}

// NoopNotifier satisfies Notifier but does nothing.
// Used when Twilio credentials are not configured.
type NoopNotifier struct{}

func (n *NoopNotifier) SendBookingConfirmation(_ context.Context, _ string, _ model.BookingDetails) error {
	return nil
}
