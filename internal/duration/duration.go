package duration

import (
	"fmt"
	"regexp"
)

type pattern struct {
	re   *regexp.Regexp
	mult float64
}

var patterns = []pattern{
	{regexp.MustCompile(`(?i)(\d+)\s*(?:час|часа|часов|ч|hour|hours|hr|hrs|h)`), 1.0 / 24.0},
	{regexp.MustCompile(`(?i)(\d+)\s*(?:день|дня|дней|д|day|days|d)`), 1},
	{regexp.MustCompile(`(?i)(\d+)\s*(?:месяц|месяца|месяцев|мес|month|months|mo|m)`), 30},
	{regexp.MustCompile(`(?i)(\d+)\s*(?:год|года|лет|year|years|yr|y)`), 365},
}
var halfYear = regexp.MustCompile(`(?i)(?:полгода|half.year|halfyear)`)

func ExtractDurations(text string) []float64 {
	out := []float64{}
	for _, p := range patterns {
		for _, m := range p.re.FindAllStringSubmatch(text, -1) {
			var n float64
			_, _ = fmtSscanf(m[1], &n)
			if n > 0 {
				out = append(out, n*p.mult)
			}
		}
	}
	if halfYear.MatchString(text) {
		out = append(out, 180)
	}
	return out
}

func fmtSscanf(s string, f *float64) (int, error) { return fmt.Sscanf(s, "%f", f) }

func ExtractTargetDuration(query string) *float64 {
	d := ExtractDurations(query)
	if len(d) == 0 {
		return nil
	}
	return &d[0]
}
func Matches(text string, target *float64, allowUnknown bool) bool {
	if target == nil {
		return true
	}
	durs := ExtractDurations(text)
	if len(durs) == 0 {
		return allowUnknown
	}
	tol := *target * 0.35
	for _, d := range durs {
		if abs(d-*target) <= tol {
			return true
		}
	}
	for _, d := range durs {
		if !(d >= *target*2 && abs(d-float64(round(d / *target))*(*target)) <= tol) {
			return false
		}
	}
	return allowUnknown
}
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
func round(f float64) int {
	if f < 0 {
		return int(f - 0.5)
	}
	return int(f + 0.5)
}
