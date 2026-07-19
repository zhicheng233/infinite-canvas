package service

const (
	MetricsHoursDefault = 24
	MetricsHoursMin     = 1
	MetricsHoursMax     = 720
)

// NormalizeMetricsHours clamps hours to [1,720], defaulting to 24.
func NormalizeMetricsHours(hours int) int {
	if hours <= 0 {
		return MetricsHoursDefault
	}
	if hours < MetricsHoursMin {
		return MetricsHoursMin
	}
	if hours > MetricsHoursMax {
		return MetricsHoursMax
	}
	return hours
}
