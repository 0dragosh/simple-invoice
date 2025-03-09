package services

import (
	"testing"
	"time"
)

func TestCalculateWorkHoursForMonth(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    time.Month
		expected float64
	}{
		{
			name:     "January 2023 (31 days, 22 workdays)",
			year:     2023,
			month:    time.January,
			expected: 22 * 8.0, // 22 workdays * 8 hours
		},
		{
			name:     "February 2023 (28 days, 20 workdays)",
			year:     2023,
			month:    time.February,
			expected: 20 * 8.0, // 20 workdays * 8 hours
		},
		{
			name:     "February 2024 (29 days, 21 workdays - leap year)",
			year:     2024,
			month:    time.February,
			expected: 21 * 8.0, // 21 workdays * 8 hours
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateWorkHoursForMonth(tt.year, tt.month)
			if result != tt.expected {
				t.Errorf("CalculateWorkHoursForMonth(%d, %v) = %v, want %v",
					tt.year, tt.month, result, tt.expected)
			}
		})
	}
}

func TestCalculateWorkHoursForCurrentMonth(t *testing.T) {
	now := time.Now()
	expected := CalculateWorkHoursForMonth(now.Year(), now.Month())

	result := CalculateWorkHoursForCurrentMonth()

	if result != expected {
		t.Errorf("CalculateWorkHoursForCurrentMonth() = %v, want %v", result, expected)
	}
}
