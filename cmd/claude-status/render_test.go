package main

import (
	"strings"
	"testing"

	"github.com/kostyay/claude-status/internal/template"
)

func TestRender(t *testing.T) {
	data := template.StatusData{Model: "Opus", Dir: "proj"}
	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{name: "custom template", tmpl: "custom {{.Dir}}", want: "custom proj"},
		{name: "parse error falls back", tmpl: "{{.Model", want: "[Opus]"},
		{name: "unknown field falls back", tmpl: "{{.NoSuchField}}", want: "[Opus]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render(tt.tmpl, data)
			if err != nil {
				t.Fatalf("render() error = %v", err)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("render() = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}
