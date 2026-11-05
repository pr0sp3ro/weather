package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pr0sp3ro/weather/internal/weather"
)

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

	r, err := weather.LoadReport(strings.TrimSpace(*city), *refresh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "weather:", err)
		os.Exit(1)
	}

	summary := weather.ShortReport(r, !*noEmoji)
	switch {
	case *waybar:
		err = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"text":    summary,
			"tooltip": weather.FullReport(r, !*noEmoji),
		})
	case *status:
		_, err = fmt.Fprintln(os.Stdout, summary)
	default:
		_, err = fmt.Fprint(os.Stdout, weather.FullReport(r, !*noEmoji))
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "weather:", err)
		os.Exit(1)
	}
}
