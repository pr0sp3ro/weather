// Package weather fetches Open-Meteo forecasts and formats weather reports.
package weather

import (
	"crypto/sha256"
	"encoding/json"
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

type Report struct {
	Place    place     `json:"place"`
	Forecast forecast  `json:"forecast"`
	Fetched  time.Time `json:"fetched"`
}

func LoadReport(city string, refresh bool) (Report, error) {
	cachePath := ""
	if dir, err := os.UserCacheDir(); err == nil {
		key := sha256.Sum256([]byte(strings.ToLower(city)))
		cachePath = filepath.Join(dir, "weather", fmt.Sprintf("v1-%x.json", key[:12]))
	}

	if !refresh && cachePath != "" {
		data, err := os.ReadFile(cachePath)
		var cached Report
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
		return Report{}, fmt.Errorf("find city: %w", err)
	}

	if len(locations.Results) == 0 {
		return Report{}, fmt.Errorf("city %q not found. Try a name qualified by country or region", city)
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
	r := Report{Place: p}
	if err := getJSON(client, "https://api.open-meteo.com/v1/forecast?"+query.Encode(), &r.Forecast); err != nil {
		return Report{}, fmt.Errorf("fetch forecast: %w", err)
	}

	if !r.Forecast.valid() {
		return Report{}, fmt.Errorf("Open-Meteo returned an incomplete forecast")
	}

	r.Fetched = time.Now()
	if cachePath != "" {
		// A missing or unwritable cache must not prevent displaying weather.
		_ = writeCache(cachePath, r)
	}

	return r, nil
}

func writeCache(path string, r Report) error {
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
