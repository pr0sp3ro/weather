package weather

import (
	"fmt"
	"strings"
	"time"
)

func item[T any](values []T, index int) (zero T) {
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return zero
}

func number(value *float64, unit string) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%s", *value, unit)
}

func condition(code *int, day bool, emoji bool) string {
	label, icon := "Unknown", "?"
	if code != nil {
		switch *code {
		case 0:
			label, icon = "Clear", "☀️"
			if !day {
				icon = "🌙"
			}
		case 1, 2:
			label, icon = "Partly cloudy", "🌤️"
		case 3:
			label, icon = "Cloudy", "☁️"
		case 45, 48:
			label, icon = "Fog", "🌫️"
		case 51, 53, 55:
			label, icon = "Drizzle", "🌦️"
		case 56, 57:
			label, icon = "Freezing drizzle", "🌧️"
		case 61, 63, 65, 80, 81, 82:
			label, icon = "Rain", "🌧️"
		case 66, 67:
			label, icon = "Freezing rain", "🌧️"
		case 71, 73, 75, 77, 85, 86:
			label, icon = "Snow", "❄️"
		case 95, 96, 99:
			label, icon = "Thunderstorm", "⛈️"
		}
	}

	if emoji {
		return icon + " " + label
	}
	return label
}

func ShortReport(r Report, emoji bool) string {
	current := r.Forecast.Current
	return fmt.Sprintf("%s %s · %s",
		condition(current.Code, current.IsDay == 1, emoji),
		number(current.Temperature, "°C"),
		r.Place.Name,
	)
}

func FullReport(r Report, emoji bool) string {
	current := r.Forecast.Current
	hourly := r.Forecast.Hourly
	daily := r.Forecast.Daily

	zone, _ := time.LoadLocation(r.Forecast.Timezone)
	formatTime := func(timestamp int64, layout string) string {
		if timestamp == 0 {
			return "-"
		}
		return time.Unix(timestamp, 0).In(zone).Format(layout)
	}

	// Select the hour containing the current observation, including after midnight.
	currentHour := -1
	for i, timestamp := range hourly.Time {
		if timestamp <= current.Time && current.Time < timestamp+3600 {
			currentHour = i
			break
		}
	}

	visibility := "-"
	if meters := item(hourly.Visibility, currentHour); meters != nil {
		visibility = fmt.Sprintf("%.1f km", *meters/1000)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Weather · %s, %s\n\n", r.Place.Name, r.Place.Country)
	fmt.Fprintf(&out, "%s  %s\n",
		condition(current.Code, current.IsDay == 1, emoji),
		number(current.Temperature, "°C"),
	)
	fmt.Fprintf(&out, "Feels like %s  ·  Low %s / High %s\n\n",
		number(current.FeelsLike, "°C"),
		number(item(daily.Min, 0), "°C"),
		number(item(daily.Max, 0), "°C"),
	)

	fmt.Fprintf(&out, "Precip   %s    Humidity   %s\n",
		number(item(hourly.Precip, currentHour), "%"),
		number(current.Humidity, "%"),
	)
	fmt.Fprintf(&out, "Wind     %s    Pressure   %s\n",
		number(current.Wind, " km/h"),
		number(current.Pressure, " hPa"),
	)
	fmt.Fprintf(&out, "UV       %s    Visibility %s\n",
		number(item(hourly.UV, currentHour), ""),
		visibility,
	)
	fmt.Fprintf(&out, "Sunrise  %s    Sunset     %s\n",
		formatTime(item(daily.Sunrise, 0), "15:04"),
		formatTime(item(daily.Sunset, 0), "15:04"),
	)

	fmt.Fprintln(&out, "\nHourly")
	shown := 0
	for i, timestamp := range hourly.Time {
		if timestamp <= current.Time {
			continue
		}

		isDay := item(hourly.IsDay, i)
		daytime := isDay != nil && *isDay == 1
		fmt.Fprintf(&out, "  %s  %5s  %4s  %s\n",
			formatTime(timestamp, "15:04"),
			number(item(hourly.Temperature, i), "°C"),
			number(item(hourly.Precip, i), "%"),
			condition(item(hourly.Code, i), daytime, emoji),
		)

		shown++
		if shown == 8 {
			break
		}
	}

	fmt.Fprintln(&out, "\nNext days")
	for day := 1; day < len(daily.Time) && day <= 5; day++ {
		fmt.Fprintf(&out, "  %s  %5s / %5s  %4s  %s\n",
			formatTime(daily.Time[day], "Mon 02"),
			number(item(daily.Min, day), "°C"),
			number(item(daily.Max, day), "°C"),
			number(item(daily.Precip, day), "%"),
			condition(item(daily.Code, day), true, emoji),
		)
	}

	fmt.Fprintf(&out, "\nUpdated %s · %s · Open-Meteo\n",
		formatTime(current.Time, "15:04"),
		r.Forecast.Timezone,
	)
	return out.String()
}
