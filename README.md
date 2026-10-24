# weather

Minimal weather in your terminal. Written in Go with no external dependencies.

Current conditions, temperature and feels like, humidity, wind, sea-level pressure,
UV, visibility, sunrise and sunset. Includes the next 8 hours and 5 days with
weather emoji, temperatures and precipitation probability.

```text
Weather · Warsaw, PL

☁️ Cloudy  9°C
Feels like 8°C  ·  Low 7°C / High 10°C

Precip   1%    Humidity   86%
Wind     7 km/h    Pressure   1018 hPa
UV       0    Visibility 50.0 km
Sunrise  07:16    Sunset     17:22

Hourly
  20:00    9°C    0%  ☁️ Cloudy
  21:00    9°C    0%  ☁️ Cloudy
  22:00    8°C    0%  🌙 Clear
  23:00    8°C    0%  🌙 Clear
  00:00    8°C    0%  ☁️ Cloudy
  01:00    7°C    0%  ☁️ Cloudy
  02:00    7°C    0%  ☁️ Cloudy
  02:00    7°C    0%  ☁️ Cloudy

Next days
  Sun 25    6°C /  12°C  100%  🌧️ Rain
  Sun 25    7°C /  10°C   87%  🌦️ Drizzle
  Mon 26    3°C /  12°C    2%  ☁️ Cloudy
  Tue 27    3°C /  12°C   12%  ☁️ Cloudy
  Wed 28    6°C /  11°C   24%  🌦️ Drizzle

Updated 19:00 · Europe/Warsaw · Open-Meteo
```

## Install

Requires Go 1.22 or newer.

```sh
go build -o weather .
install -Dm755 weather ~/.local/bin/weather
```

Make sure `~/.local/bin` is on your `PATH`.

## Usage

```sh
weather 'Warsaw, PL'
weather --city 'London, GB'
weather --no-emoji 'Berlin, DE'
weather --status 'Warsaw, PL'
weather --refresh 'Warsaw, PL'
weather --help
```

Set a default city in your shell or statusbar environment:

```sh
export WEATHER_CITY='Warsaw, PL'
weather
```

Use a country or region to distinguish cities with the same name. The first
matching location is used and displayed in the full view. All times are local to
that location. Units are Celsius, km/h, hPa and km. Percentages indicate the chance
of precipitation, including snow. `-` means data is unavailable.

Successful results are cached for 15 minutes in `$XDG_CACHE_HOME/weather`
(normally `~/.cache/weather`). `--refresh` bypasses the cache. Network errors are
written to stderr with a nonzero exit status. Expired data is not displayed.

## Statusbar

`weather --status` prints one line with conditions, temperature and city for
scripts, tmux, i3status or other bars. Set `WEATHER_CITY` or pass a city explicitly.

Waybar supports a full forecast tooltip with `--waybar`. Add `custom/weather` to
your module list and configure it:

```json
"custom/weather": {
  "exec": "weather --waybar --city 'Warsaw, PL'",
  "return-type": "json",
  "interval": 900,
  "escape": true
}
```

Add `--no-emoji` if your font does not support weather emoji.

## Data and license

Weather by [Open-Meteo](https://open-meteo.com/), location search based on
[GeoNames](https://www.geonames.org/). Data is provided under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). The public API requires
no key for non-commercial use. See [API terms](https://open-meteo.com/en/terms)
for usage limits and commercial use.

Code: [MIT](LICENSE), copyright 2026 [Synframe Labs](https://synframe.eu).
