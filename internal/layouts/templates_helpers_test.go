package layouts

import "testing"

func TestGetValueOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		values     []string
		index      int
		defaultVal string
		expected   string
	}{
		{"value exists", []string{"hello"}, 0, "default", "hello"},
		{"value is empty", []string{""}, 0, "default", "default"},
		{"index out of range", []string{"a"}, 5, "default", "default"},
		{"empty slice", []string{}, 0, "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrDefault(tt.values, tt.index, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single line", "hello", "hello"},
		{"multiple lines", "line1\nline2", "line1\nline2"},
		{"with whitespace", "  line1  \n  line2  ", "line1\nline2"},
		{"empty lines", "line1\n\nline2", "line1\n\nline2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatContent(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
