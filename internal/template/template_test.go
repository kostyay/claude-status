package template

import (
	"testing"
)

func TestNewEngine_InvalidTemplate(t *testing.T) {
	tmpl := `{{.Model` // Missing closing braces
	_, err := NewEngine(tmpl)
	if err == nil {
		t.Fatal("NewEngine() expected error for invalid template")
	}
}

func TestRender_AllFields(t *testing.T) {
	tmpl := `[{{.Model}}] {{.Dir}} {{.GitBranch}} {{.GitStatus}} {{.GitHubStatus}} v{{.Version}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{
		Model:        "Claude",
		Dir:          "myproject",
		GitBranch:    "main",
		GitStatus:    "±3",
		GitHubStatus: "✅",
		Version:      "1.0.0",
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "[Claude] myproject main ±3 ✅ v1.0.0"
	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_EmptyOptionals(t *testing.T) {
	tmpl := `[{{.Model}}]{{if .GitBranch}} | {{.GitBranch}}{{end}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{if .GitHubStatus}} | {{.GitHubStatus}}{{end}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{
		Model: "Claude",
		// All optional fields empty
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "[Claude]"
	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestColorFunctions(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		want     string
	}{
		{"cyan", "cyan", "\033[36m"},
		{"blue", "blue", "\033[34m"},
		{"green", "green", "\033[32m"},
		{"yellow", "yellow", "\033[33m"},
		{"red", "red", "\033[31m"},
		{"gray", "gray", "\033[90m"},
		{"reset", "reset", "\033[0m"},
		{"bold", "bold", "\033[1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{` + tt.funcName + `}}`
			engine, err := NewEngine(tmpl)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}

			result, err := engine.Render(StatusData{})
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if result != tt.want {
				t.Errorf("%s() = %q, want %q", tt.funcName, result, tt.want)
			}
		})
	}
}

func TestCtxColorFunction(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		wantColor  string
	}{
		{"0% - green", 0, "\033[32m"},
		{"49.9% - green", 49.9, "\033[32m"},
		{"50% - yellow", 50, "\033[33m"},
		{"79.9% - yellow", 79.9, "\033[33m"},
		{"80% - red", 80, "\033[31m"},
		{"100% - red", 100, "\033[31m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{ctxColor .ContextPct}}`
			engine, err := NewEngine(tmpl)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}

			data := StatusData{ContextPct: tt.percentage}
			result, err := engine.Render(data)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if result != tt.wantColor {
				t.Errorf("ctxColor(%v) = %q, want %q", tt.percentage, result, tt.wantColor)
			}
		})
	}
}

func TestFmtCostFunction(t *testing.T) {
	tests := []struct {
		name string
		usd  float64
		want string
	}{
		{"zero", 0, "$0.00"},
		{"small amount", 0.01234, "$0.01"},
		{"one dollar fifty", 1.5, "$1.50"},
		{"larger amount", 12.345, "$12.35"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{fmtCost .CostUSD}}`
			engine, err := NewEngine(tmpl)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}

			data := StatusData{CostUSD: tt.usd}
			result, err := engine.Render(data)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if result != tt.want {
				t.Errorf("fmtCost(%v) = %q, want %q", tt.usd, result, tt.want)
			}
		})
	}
}

func TestFmtDurationFunction(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero", 0, "0s"},
		{"five seconds", 5000, "5s"},
		{"one minute", 60000, "1m 0s"},
		{"two minutes five seconds", 125000, "2m 5s"},
		{"ten minutes", 600000, "10m 0s"},
		{"one hour", 3600000, "1h 0m"},
		{"one hour thirty minutes", 5400000, "1h 30m"},
		{"two hours five minutes", 7500000, "2h 5m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{fmtDuration .DurationMS}}`
			engine, err := NewEngine(tmpl)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}

			data := StatusData{DurationMS: tt.ms}
			result, err := engine.Render(data)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if result != tt.want {
				t.Errorf("fmtDuration(%v) = %q, want %q", tt.ms, result, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		count int64
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1k"},
		{1234, "1.2k"},
		{10500, "10.5k"},
		{999900, "999.9k"},
		{1000000, "1M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatTokens(tt.count); got != tt.want {
				t.Errorf("FormatTokens(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestFmtPctAndSigned(t *testing.T) {
	tmpl := `{{fmtPct .ContextPct}} {{fmtSigned .CostLinesAdded}}/{{fmtSigned .CostLinesRemoved}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	got, err := engine.Render(StatusData{ContextPct: 75, CostLinesAdded: 42, CostLinesRemoved: -15})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if want := "75.0% +42/-15"; got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}
