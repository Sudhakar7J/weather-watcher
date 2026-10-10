package weather

import "fmt"

// wmoDescriptions maps the WMO weather interpretation codes used by
// Open-Meteo to human-readable text. Keys are the codes defined in the WMO
// Manual on Codes, table 4677.
var wmoDescriptions = map[int]string{
	0:  "Clear sky",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Depositing rime fog",

	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	56: "Light freezing drizzle",
	57: "Dense freezing drizzle",

	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	66: "Light freezing rain",
	67: "Heavy freezing rain",

	71: "Slight snowfall",
	73: "Moderate snowfall",
	75: "Heavy snowfall",
	77: "Snow grains",

	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	85: "Slight snow showers",
	86: "Heavy snow showers",

	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

// DescribeCode returns a human-readable description for a WMO weather code.
// Unknown codes fall back to a generic message rather than an empty string, so
// the UI never renders a blank label.
func DescribeCode(code int) string {
	if desc, ok := wmoDescriptions[code]; ok {
		return desc
	}
	return fmt.Sprintf("Unknown conditions (code %d)", code)
}
