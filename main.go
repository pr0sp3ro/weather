package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"
)

type place struct {
	Name      string  `json:"name"`
	Country   string  `json:"country_code"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type forecast struct {
	Timezone string `json:"timezone"`
	Current  struct {
		Time        int64    `json:"time"`
		Temperature *float64 `json:"temperature_2m"`
		FeelsLike   *float64 `json:"apparent_temperature"`
		Humidity    *float64 `json:"relative_humidity_2m"`
		Wind        *float64 `json:"wind_speed_10m"`
		Pressure    *float64 `json:"pressure_msl"`
		Code        *int     `json:"weather_code"`
		IsDay       int      `json:"is_day"`
	} `json:"current"`
	Hourly struct {
		Time        []int64    `json:"time"`
		Temperature []*float64 `json:"temperature_2m"`
		Precip      []*float64 `json:"precipitation_probability"`
		Visibility  []*float64 `json:"visibility"`
		UV          []*float64 `json:"uv_index"`
		Code        []*int     `json:"weather_code"`
		IsDay       []*int     `json:"is_day"`
	} `json:"hourly"`
	Daily struct {
		Time    []int64    `json:"time"`
		Min     []*float64 `json:"temperature_2m_min"`
		Max     []*float64 `json:"temperature_2m_max"`
		Precip  []*float64 `json:"precipitation_probability_max"`
		Code    []*int     `json:"weather_code"`
		Sunrise []int64    `json:"sunrise"`
		Sunset  []int64    `json:"sunset"`
	} `json:"daily"`
}

type report struct {
	Place    place     `json:"place"`
	Forecast forecast  `json:"forecast"`
	Fetched  time.Time `json:"fetched"`
}

func main() {
	city := flag.String("city", os.Getenv("WEATHER_CITY"), "City, optionally qualified by country or region")
	status := flag.Bool("status", false, "Print one line for a statusbar")
	waybar := flag.Bool("waybar", false, "Print Waybar JSON with a forecast tooltip")
	refresh := flag.Bool("refresh", false, "Bypass the 15-minute cache")
	noEmoji := flag.Bool("no-emoji", false, "Use text without emoji")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: weather [options] [city]\n\nMinimal weather from Open-Meteo.")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() > 0 {
		*city = strings.Join(flag.Args(), " ")
	}

	if strings.TrimSpace(*city) == "" {
		fmt.Fprintln(os.Stderr, "weather: specify a city, e.g. weather 'Warsaw, PL', or set WEATHER_CITY")
		os.Exit(1)
	}

	r, err := loadReport(strings.TrimSpace(*city), *refresh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "weather:", err)
		os.Exit(1)
	}

	summary := shortReport(r, !*noEmoji)
	switch {
	case *waybar:
		err = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"text":    summary,
			"tooltip": fullReport(r, !*noEmoji),
		})
	case *status:
		_, err = fmt.Fprintln(os.Stdout, summary)
	default:
		_, err = fmt.Fprint(os.Stdout, fullReport(r, !*noEmoji))
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "weather:", err)
		os.Exit(1)
	}
}

func loadReport(city string, refresh bool) (report, error) {
	cachePath := ""
	if dir, err := os.UserCacheDir(); err == nil {
		key := sha256.Sum256([]byte(strings.ToLower(city)))
		cachePath = filepath.Join(dir, "weather", fmt.Sprintf("v1-%x.json", key[:12]))
	}

	if !refresh && cachePath != "" {
		data, err := os.ReadFile(cachePath)
		var cached report
		if err == nil && json.Unmarshal(data, &cached) == nil {
			age := time.Since(cached.Fetched)
			if age >= 0 && age < 15*time.Minute && cached.Forecast.valid() {
				return cached, nil
			}
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var locations struct {
		Results []place `json:"results"`
	}
	query := url.Values{"name": {city}, "count": {"1"}, "language": {"en"}}
	if err := getJSON(client, "https://geocoding-api.open-meteo.com/v1/search?"+query.Encode(), &locations); err != nil {
		return report{}, fmt.Errorf("find city: %w", err)
	}

	if len(locations.Results) == 0 {
		return report{}, fmt.Errorf("city %q not found. Try a name qualified by country or region", city)
	}

	p := locations.Results[0]
	query = url.Values{
		"latitude":         {fmt.Sprint(p.Latitude)},
		"longitude":        {fmt.Sprint(p.Longitude)},
		"timezone":         {"auto"},
		"timeformat":       {"unixtime"},
		"forecast_days":    {"6"},
		"temperature_unit": {"celsius"},
		"wind_speed_unit":  {"kmh"},
		"current":          {"temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,pressure_msl,weather_code,is_day"},
		"hourly":           {"temperature_2m,precipitation_probability,visibility,uv_index,weather_code,is_day"},
		"daily":            {"temperature_2m_min,temperature_2m_max,precipitation_probability_max,weather_code,sunrise,sunset"},
	}
	r := report{Place: p}
	if err := getJSON(client, "https://api.open-meteo.com/v1/forecast?"+query.Encode(), &r.Forecast); err != nil {
		return report{}, fmt.Errorf("fetch forecast: %w", err)
	}

	if !r.Forecast.valid() {
		return report{}, fmt.Errorf("Open-Meteo returned an incomplete forecast")
	}

	r.Fetched = time.Now()
	if cachePath != "" {
		// A missing or unwritable cache must not prevent displaying weather.
		_ = writeCache(cachePath, r)
	}

	return r, nil
}

func writeCache(path string, r report) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	file, err := os.CreateTemp(dir, ".weather-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}

	if closeErr != nil {
		return closeErr
	}

	// Rename a temporary file so concurrent statusbar reads see complete JSON.
	return os.Rename(file.Name(), path)
}

func getJSON(client *http.Client, endpoint string, target any) error {
	res, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Open-Meteo: %s", res.Status)
	}

	return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(target)
}

func (f forecast) valid() bool {
	_, err := time.LoadLocation(f.Timezone)
	return f.Timezone != "" && err == nil && f.Current.Time > 0 &&
		f.Current.Temperature != nil && f.Current.Code != nil &&
		len(f.Hourly.Time) > 0 && len(f.Daily.Time) > 0
}

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

func shortReport(r report, emoji bool) string {
	current := r.Forecast.Current
	return fmt.Sprintf("%s %s · %s",
		condition(current.Code, current.IsDay == 1, emoji),
		number(current.Temperature, "°C"),
		r.Place.Name,
	)
}

func fullReport(r report, emoji bool) string {
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
