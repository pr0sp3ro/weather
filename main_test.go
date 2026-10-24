package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReportAcrossMidnight(t *testing.T) {
	var f forecast
	err := json.Unmarshal([]byte(`{
		"timezone": "Europe/Warsaw",
		"current": {"time": 1792879500, "temperature_2m": 5, "weather_code": 3},
		"hourly": {
			"time": [1792879200, 1792882800],
			"temperature_2m": [5, 4],
			"precipitation_probability": [20, 40],
			"weather_code": [3, 61]
		},
		"daily": {"time": [1792792800, 1792879200]}
	}`), &f)
	if err != nil {
		t.Fatal(err)
	}

	// Pin the observation at 00:05 local and the forecast at 00:00 and 01:00.
	zone, err := time.LoadLocation(f.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 25, 0, 0, 0, 0, zone)
	f.Current.Time = start.Add(5 * time.Minute).Unix()
	f.Hourly.Time = []int64{start.Unix(), start.Add(time.Hour).Unix()}
	f.Daily.Time = []int64{start.Unix(), start.AddDate(0, 0, 1).Unix()}
	output := fullReport(report{Forecast: f, Place: place{Name: "Warsaw", Country: "PL"}}, false)
	for _, want := range []string{"Precip   20%", "01:00", "40%", "Rain", "Humidity   -", "Visibility -"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in report:\n%s", want, output)
		}
	}
	if strings.Contains(output, "  00:00") {
		t.Errorf("hourly forecast includes an elapsed hour:\n%s", output)
	}
}

func TestRejectMissingCurrentConditions(t *testing.T) {
	for _, input := range []string{
		`{}`,
		`{"timezone":"Europe/Warsaw","current":{"time":1,"temperature_2m":null,"weather_code":0},"hourly":{"time":[1]},"daily":{"time":[1]}}`,
	} {
		var f forecast
		if err := json.Unmarshal([]byte(input), &f); err != nil {
			t.Fatal(err)
		}
		if f.valid() {
			t.Errorf("accepted incomplete forecast: %s", input)
		}
	}
}
