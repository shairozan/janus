//go:build unit
// +build unit

package gui

import (
	"reflect"
	"testing"
)

func TestParseNonmemOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "single option",
			input:    "-maxeval=9999",
			expected: []string{"-maxeval=9999"},
		},
		{
			name:     "multiple options",
			input:    "-maxeval=9999 -files=100",
			expected: []string{"-maxeval=9999", "-files=100"},
		},
		{
			name:     "options with extra spaces",
			input:    "  -maxeval=9999    -files=100  ",
			expected: []string{"-maxeval=9999", "-files=100"},
		},
		{
			name:     "complex options",
			input:    "-maxeval=9999 -files=100 -sigdigits=4",
			expected: []string{"-maxeval=9999", "-files=100", "-sigdigits=4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNonmemOptions(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseNonmemOptions(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}
