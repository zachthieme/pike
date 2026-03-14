package style

import (
	"testing"

	"pike/internal/model"
)

func TestTagToken(t *testing.T) {
	tests := []struct {
		name string
		tag  model.Tag
		want string
	}{
		{"bare tag", model.Tag{Name: "today"}, "@today"},
		{"valued tag", model.Tag{Name: "due", Value: "2026-03-15"}, "@due(2026-03-15)"},
		{"empty value treated as bare", model.Tag{Name: "risk", Value: ""}, "@risk"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagToken(tt.tag); got != tt.want {
				t.Errorf("TagToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ANSI", "hello world", "hello world"},
		{"simple color", "\033[31mred\033[0m", "red"},
		{"24-bit color", "\033[38;2;255;87;51mhex\033[0m", "hex"},
		{"mixed", "before \033[32mgreen\033[0m after", "before green after"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripANSI(tt.input); got != tt.want {
				t.Errorf("StripANSI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestANSIStyleFunc(t *testing.T) {
	sf := ANSIStyleFunc()
	tests := []struct {
		name  string
		text  string
		color string
		want  string
	}{
		{"named color", "@today", "green", "\033[32m@today\033[0m"},
		{"hex color", "@special", "#FF5733", "\033[38;2;255;87;51m@special\033[0m"},
		{"unknown color returns text unchanged", "@tag", "nope", "@tag"},
		{"empty color returns text unchanged", "@tag", "", "@tag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sf(tt.text, tt.color); got != tt.want {
				t.Errorf("ANSIStyleFunc() = %q, want %q", got, tt.want)
			}
		})
	}
}
