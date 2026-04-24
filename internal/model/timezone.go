package model

import "time"

// BusinessLocation resolves a business's IANA timezone string to a
// *time.Location. Malformed or missing strings fall back to UTC; the DB
// default guarantees a valid value for new signups, but legacy rows (or a
// bad Intl.DateTimeFormat detection) shouldn't 500 the whole flow.
func BusinessLocation(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}
