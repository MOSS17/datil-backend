package model

import (
	"time"

	"github.com/google/uuid"
)

type Business struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Location         *string   `json:"location"`
	Description      *string   `json:"description"`
	LogoURL          *string   `json:"logo_url"`
	URL              string    `json:"url"`
	Timezone         string    `json:"timezone"`
	BeneficiaryClabe *string   `json:"beneficiary_clabe"`
	BankName         *string   `json:"bank_name"`
	BeneficiaryName  *string   `json:"beneficiary_name"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type User struct {
	ID         uuid.UUID `json:"id"`
	BusinessID uuid.UUID `json:"business_id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Password   string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Category struct {
	ID            uuid.UUID `json:"id"`
	BusinessID    uuid.UUID `json:"business_id"`
	Name          string    `json:"name"`
	AllowMultiple bool      `json:"allow_multiple"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Service struct {
	ID                   uuid.UUID `json:"id"`
	BusinessID           uuid.UUID `json:"business_id"`
	CategoryID           uuid.UUID `json:"category_id"`
	Name                 string    `json:"name"`
	Description          *string   `json:"description"`
	MinPrice             float64   `json:"min_price"`
	MaxPrice             *float64  `json:"max_price"`
	Duration             int       `json:"duration"`
	AdvancePaymentAmount *float64  `json:"advance_payment_amount"`
	IsExtra              bool      `json:"is_extra"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ServiceExtra struct {
	ServiceID uuid.UUID `json:"service_id"`
	ExtraID   uuid.UUID `json:"extra_id"`
}

type Appointment struct {
	ID                     uuid.UUID            `json:"id"`
	UserID                 uuid.UUID            `json:"user_id"`
	CustomerName           string               `json:"customer_name"`
	CustomerEmail          *string              `json:"customer_email"`
	StartTime              time.Time            `json:"start_time"`
	EndTime                time.Time            `json:"end_time"`
	Total                  float64              `json:"total"`
	CustomerPhone          string               `json:"customer_phone"`
	AdvancePaymentImageURL *string              `json:"advance_payment_image_url"`
	Status                 string               `json:"status"`
	GoogleEventID          *string              `json:"google_event_id,omitempty"`
	IcalSequence           int                  `json:"ical_sequence"`
	SeenAt                 *time.Time           `json:"seen_at"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
	Services               []AppointmentService `json:"services,omitempty"`
}

type AppointmentService struct {
	AppointmentID uuid.UUID `json:"appointment_id"`
	ServiceID     uuid.UUID `json:"service_id"`
	Price         float64   `json:"price"`
	Duration      int       `json:"duration"`
}

type Workday struct {
	ID         uuid.UUID  `json:"id"`
	BusinessID uuid.UUID  `json:"business_id"`
	Day        int        `json:"day"`
	IsEnabled  bool       `json:"is_enabled"`
	Hours      []WorkHour `json:"hours"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type WorkHour struct {
	ID        uuid.UUID `json:"id"`
	DayID     uuid.UUID `json:"workday_id"`
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PersonalTime struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	StartTime *string    `json:"start_time"`
	EndTime   *string    `json:"end_time"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CalendarIntegration struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	Provider     string     `json:"provider"`
	AccessToken  *string    `json:"-"`
	RefreshToken *string    `json:"-"`
	AccountEmail *string    `json:"account_email,omitempty"`
	FeedToken    *string    `json:"-"`
	IsActive     bool       `json:"is_active"`
	ExpiresAt    *time.Time `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Request/response types

type SignupRequest struct {
	Name         string `json:"name"`
	BusinessName string `json:"business_name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	// Timezone is optional; frontend auto-detects via
	// Intl.DateTimeFormat().resolvedOptions().timeZone and sends an IANA
	// name. If omitted or invalid, repository.DefaultBusinessTimezone
	// applies. Validated in the signup handler via time.LoadLocation.
	Timezone string `json:"timezone,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UpdateBusinessRequest struct {
	Name        string  `json:"name"`
	Location    *string `json:"location"`
	Description *string `json:"description"`
}

type UpdateBankRequest struct {
	BeneficiaryClabe string `json:"beneficiary_clabe"`
	BankName         string `json:"bank_name"`
	BeneficiaryName  string `json:"beneficiary_name"`
}

type ServiceRequest struct {
	CategoryID           uuid.UUID `json:"category_id"`
	Name                 string    `json:"name"`
	Description          *string   `json:"description"`
	MinPrice             float64   `json:"min_price"`
	MaxPrice             *float64  `json:"max_price"`
	Duration             int       `json:"duration"`
	AdvancePaymentAmount *float64  `json:"advance_payment_amount"`
	IsExtra              bool      `json:"is_extra"`
	IsActive             *bool     `json:"is_active"`
}

type LinkExtraRequest struct {
	ExtraID uuid.UUID `json:"extra_id"`
}

type CategoryRequest struct {
	Name          string `json:"name"`
	AllowMultiple bool   `json:"allow_multiple"`
}

type BookingPageResponse struct {
	Business   Business   `json:"business"`
	Categories []Category `json:"categories"`
}

type BookingService struct {
	Service
	Extras []Service `json:"extras"`
}

type BookingDetails struct {
	CustomerName string    `json:"customer_name"`
	BusinessName string    `json:"business_name"`
	StartTime    time.Time `json:"start_time"`
	Services     []string  `json:"services"`
}

type DashboardData struct {
	TodayCount    int           `json:"today_count"`
	WeekCount     int           `json:"week_count"`
	MonthlyIncome float64       `json:"monthly_income"`
	Upcoming      []Appointment `json:"upcoming"`
	Latest        []Appointment `json:"latest"`
}

type CreateAppointmentRequest struct {
	CustomerName  string      `json:"customer_name"`
	CustomerPhone string      `json:"customer_phone"`
	CustomerEmail *string     `json:"customer_email"`
	StartTime     time.Time   `json:"start_time"`
	ServiceIDs    []uuid.UUID `json:"service_ids"`
	ExtraIDs      []uuid.UUID `json:"extra_ids"`
}

type UpdateAppointmentRequest struct {
	CustomerName  string    `json:"customer_name"`
	CustomerEmail *string   `json:"customer_email"`
	CustomerPhone string    `json:"customer_phone"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Total         float64   `json:"total"`
}

type UpdateAppointmentStatusRequest struct {
	Status string `json:"status"`
}

// CreatePersonalTimeRequest accepts the shape the frontend sends for all
// three UI patterns: a single "hours" block, a "full_day", or a "date_range".
// type/reason/date are declared so DisallowUnknownFields doesn't reject them;
// the handler normalises to start_date/end_date/start_time/end_time before
// calling the repo. Persisting type/reason would need a schema change.
type CreatePersonalTimeRequest struct {
	Type      string  `json:"type,omitempty"`
	Reason    string  `json:"reason,omitempty"`
	Date      string  `json:"date,omitempty"`
	StartDate string  `json:"start_date,omitempty"`
	EndDate   string  `json:"end_date,omitempty"`
	StartTime *string `json:"start_time,omitempty"`
	EndTime   *string `json:"end_time,omitempty"`
}
