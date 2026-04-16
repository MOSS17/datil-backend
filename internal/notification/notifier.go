package notification

import (
	"context"

	"github.com/mossandoval/datil-api/internal/model"
)

type Notifier interface {
	SendBookingConfirmation(ctx context.Context, phone string, details model.BookingDetails) error
}
