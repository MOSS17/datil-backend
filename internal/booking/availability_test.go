package booking

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mossandoval/datil-api/internal/model"
)

func mustDay(t *testing.T) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("loading location: %v", err)
	}
	return time.Date(2026, time.April, 23, 0, 0, 0, 0, loc)
}

func workdayWith(t *testing.T, hours ...[2]string) model.Workday {
	t.Helper()
	wh := make([]model.WorkHour, 0, len(hours))
	for _, h := range hours {
		wh = append(wh, model.WorkHour{
			ID:        uuid.New(),
			StartTime: h[0],
			EndTime:   h[1],
		})
	}
	return model.Workday{
		ID:        uuid.New(),
		IsEnabled: true,
		Hours:     wh,
	}
}

func appt(day time.Time, startHHMM, endHHMM string) model.Appointment {
	parse := func(s string) time.Time {
		t, _ := time.ParseInLocation("15:04", s, day.Location())
		return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, day.Location())
	}
	return model.Appointment{
		ID:        uuid.New(),
		StartTime: parse(startHHMM),
		EndTime:   parse(endHHMM),
	}
}

func ptr(s string) *string { return &s }

// startTimes returns the slot starts as "HH:MM" for compact assertions.
func startTimes(slots []TimeSlot) []string {
	out := make([]string, 0, len(slots))
	for _, s := range slots {
		out = append(out, s.Start.Format("15:04"))
	}
	return out
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestComputeSlots(t *testing.T) {
	t.Parallel()

	day := mustDay(t)

	cases := []struct {
		name     string
		workday  model.Workday
		personal []model.PersonalTime
		appts    []model.Appointment
		duration int
		step     int
		want     []string
	}{
		{
			name:     "single window no blocks",
			workday:  workdayWith(t, [2]string{"09:00", "11:00"}),
			duration: 30,
			step:     15,
			want: []string{
				"09:00", "09:15", "09:30", "09:45",
				"10:00", "10:15", "10:30",
			},
		},
		{
			name:     "lunch break (two windows)",
			workday:  workdayWith(t, [2]string{"09:00", "10:00"}, [2]string{"13:00", "14:00"}),
			duration: 30,
			step:     30,
			want:     []string{"09:00", "09:30", "13:00", "13:30"},
		},
		{
			name:    "appointment mid-day carves the window",
			workday: workdayWith(t, [2]string{"09:00", "12:00"}),
			appts: []model.Appointment{
				appt(day, "10:00", "10:30"),
			},
			duration: 30,
			step:     30,
			want:     []string{"09:00", "09:30", "10:30", "11:00", "11:30"},
		},
		{
			name:    "personal time half-day",
			workday: workdayWith(t, [2]string{"09:00", "17:00"}),
			personal: []model.PersonalTime{
				{
					StartDate: day.Format("2006-01-02"),
					EndDate:   day.Format("2006-01-02"),
					StartTime: ptr("12:00"),
					EndTime:   ptr("17:00"),
				},
			},
			duration: 60,
			step:     60,
			want:     []string{"09:00", "10:00", "11:00"},
		},
		{
			name:     "duration exceeds all windows returns empty",
			workday:  workdayWith(t, [2]string{"09:00", "10:00"}, [2]string{"13:00", "14:00"}),
			duration: 90,
			step:     15,
			want:     []string{},
		},
		{
			name:     "boundary slot ends exactly at window close",
			workday:  workdayWith(t, [2]string{"09:00", "09:30"}),
			duration: 30,
			step:     15,
			want:     []string{"09:00"},
		},
		{
			name:     "disabled workday returns empty",
			workday:  model.Workday{IsEnabled: false},
			duration: 30,
			step:     15,
			want:     []string{},
		},
		{
			name:    "all-day personal time clears the day",
			workday: workdayWith(t, [2]string{"09:00", "17:00"}),
			personal: []model.PersonalTime{
				{
					StartDate: day.Format("2006-01-02"),
					EndDate:   day.Format("2006-01-02"),
					// StartTime/EndTime nil → all-day block.
				},
			},
			duration: 30,
			step:     15,
			want:     []string{},
		},
		{
			name:    "appointment outside the day is ignored",
			workday: workdayWith(t, [2]string{"09:00", "10:00"}),
			appts: []model.Appointment{
				appt(day.AddDate(0, 0, 1), "09:00", "09:30"),
			},
			duration: 30,
			step:     30,
			want:     []string{"09:00", "09:30"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := startTimes(ComputeSlots(
				tc.workday, tc.personal, tc.appts, tc.duration, day, tc.step,
			))
			if !sliceEq(got, tc.want) {
				t.Fatalf("starts mismatch:\n  got:  %v\n  want: %v", got, tc.want)
			}
		})
	}
}
