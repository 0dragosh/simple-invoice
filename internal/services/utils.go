package services

import (
	"time"
)

// CalculateWorkHoursForMonth calculates the total work hours for a given month
// excluding weekends (8 hours per day, 5 days per week)
func CalculateWorkHoursForMonth(year int, month time.Month) float64 {
	// Get the first day of the month
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Get the last day of the month
	lastDay := firstDay.AddDate(0, 1, -1)

	var totalHours float64 = 0

	// Iterate through each day of the month
	for day := firstDay; day.Before(lastDay.AddDate(0, 0, 1)); day = day.AddDate(0, 0, 1) {
		// Check if the day is a weekday (Monday to Friday)
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			totalHours += 8.0 // 8 hours per workday
		}
	}

	return totalHours
}

// CalculateWorkHoursForCurrentMonth calculates the total work hours for the current month
// excluding weekends (8 hours per day, 5 days per week)
func CalculateWorkHoursForCurrentMonth() float64 {
	now := time.Now()
	return CalculateWorkHoursForMonth(now.Year(), now.Month())
}
