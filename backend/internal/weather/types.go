package weather

import "time"

// Report is the weather for one city at a point in time.
//
// A Report is cached and therefore shared between requests, so it must be
// treated as immutable once returned. Callers that need to vary metadata per
// request (such as whether it came from cache) copy the struct first; that copy
// is safe because the Hourly slice is only ever read.
type Report struct {
	City      string  `json:"city"`
	Country   string  `json:"country,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`

	Current Current       `json:"current"`
	Hourly  []HourlyPoint `json:"hourly"`

	FetchedAt time.Time `json:"fetched_at"`

	// FromCache reports whether this response was served from the in-memory
	// cache. It is set by the service layer on a copy, never on the cached
	// value itself.
	FromCache bool `json:"from_cache"`
}

// Current holds conditions at the observation time.
type Current struct {
	TemperatureC float64   `json:"temperature_c"`
	FeelsLikeC   float64   `json:"feels_like_c"`
	HumidityPct  float64   `json:"humidity_pct"`
	WindSpeedKph float64   `json:"wind_speed_kph"`
	Code         int       `json:"code"`
	Description  string    `json:"description"`
	ObservedAt   time.Time `json:"observed_at"`
}

// HourlyPoint is a single hour on the temperature trend used by the frontend
// chart.
type HourlyPoint struct {
	Time         time.Time `json:"time"`
	TemperatureC float64   `json:"temperature_c"`
}
