package template

import (
	"strings"
	"testing"
)

func TestNewEngine_ValidTemplate(t *testing.T) {
	tmpl := `{{.Model}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine() returned nil engine")
	}
}

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

func TestRender_ColorFunctions(t *testing.T) {
	tmpl := `{{cyan}}test{{reset}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	result, err := engine.Render(StatusData{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result, "\033[36m") {
		t.Error("Render() missing cyan color code")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("Render() missing reset color code")
	}
	if !strings.Contains(result, "test") {
		t.Error("Render() missing content")
	}
}

func TestRender_CustomTemplate(t *testing.T) {
	// A completely custom template
	tmpl := `Model: {{.Model}} | Branch: {{.GitBranch}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{
		Model:     "Claude",
		GitBranch: "feature",
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "Model: Claude | Branch: feature"
	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_EscapedBraces(t *testing.T) {
	// Test that we can include literal text
	tmpl := `{{.Model}} - literal text`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{Model: "Claude"}
	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "Claude - literal text"
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

func TestRender_ComplexTemplate(t *testing.T) {
	// Test the default template style
	tmpl := `{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .GitHubStatus}} | {{.GitHubStatus}}{{end}}{{if .Version}} | {{gray}}v{{.Version}}{{reset}}{{end}}`

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

	// Check key parts are present
	if !strings.Contains(result, "[Claude]") {
		t.Error("Missing model name")
	}
	if !strings.Contains(result, "📁 myproject") {
		t.Error("Missing directory")
	}
	if !strings.Contains(result, "🌿 main") {
		t.Error("Missing git branch")
	}
	if !strings.Contains(result, "±3") {
		t.Error("Missing git status")
	}
	if !strings.Contains(result, "✅") {
		t.Error("Missing GitHub status")
	}
	if !strings.Contains(result, "v1.0.0") {
		t.Error("Missing version")
	}
}

func TestRender_PrefixField(t *testing.T) {
	tmpl := `{{if .Prefix}}{{.Prefix}} | {{end}}[{{.Model}}]`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Test with prefix
	data := StatusData{
		Prefix: "WORK",
		Model:  "Claude",
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "WORK | [Claude]"
	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_EmptyPrefix(t *testing.T) {
	tmpl := `{{if .Prefix}}{{.Prefix}} | {{end}}[{{.Model}}]`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	// Test without prefix
	data := StatusData{
		Prefix: "",
		Model:  "Claude",
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Should not have prefix separator
	expected := "[Claude]"
	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_PrefixColor(t *testing.T) {
	// Test that PrefixColor field applies the color directly
	tmpl := `{{if .Prefix}}{{.PrefixColor}}{{.Prefix}}{{reset}} | {{end}}[{{.Model}}]`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{
		Prefix:      "WORK",
		PrefixColor: ColorMap["red"],
		Model:       "Claude",
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Should have the red color code
	if !strings.Contains(result, "\033[31m") {
		t.Errorf("Missing red color code, got: %q", result)
	}
	if !strings.Contains(result, "WORK") {
		t.Error("Missing WORK text in prefix")
	}
	if !strings.Contains(result, "[Claude]") {
		t.Error("Missing model name")
	}
}

func TestColorMap(t *testing.T) {
	// Verify ColorMap contains expected colors
	expectedColors := []string{"cyan", "blue", "green", "yellow", "red", "magenta", "gray"}
	for _, name := range expectedColors {
		if _, ok := ColorMap[name]; !ok {
			t.Errorf("ColorMap missing color: %s", name)
		}
	}

	// Verify colors are ANSI codes
	for name, code := range ColorMap {
		if !strings.HasPrefix(code, "\033[") {
			t.Errorf("ColorMap[%s] = %q, want ANSI code starting with \\033[", name, code)
		}
	}
}

func TestCtxColorFunction(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		wantColor  string
	}{
		{"0% - green", 0, "\033[32m"},
		{"25% - green", 25, "\033[32m"},
		{"49.9% - green", 49.9, "\033[32m"},
		{"50% - yellow", 50, "\033[33m"},
		{"65% - yellow", 65, "\033[33m"},
		{"79.9% - yellow", 79.9, "\033[33m"},
		{"80% - red", 80, "\033[31m"},
		{"90% - red", 90, "\033[31m"},
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

func TestRender_ContextPercentageWithColor(t *testing.T) {
	// Test a template using ctxColor with context percentage
	tmpl := `{{ctxColor .ContextPct}}📊 {{fmtPct .ContextPct}}{{reset}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{
		ContextPct: 75.0,
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Should have yellow color (50-80%)
	if !strings.Contains(result, "\033[33m") {
		t.Error("Missing yellow color code for 75%")
	}
	if !strings.Contains(result, "📊 75.0%") {
		t.Error("Missing context percentage")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("Missing reset code")
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

func TestRender_CostTemplate(t *testing.T) {
	tmpl := `{{if .CostUSD}}💰 {{fmtCost .CostUSD}}{{end}}{{if .DurationMS}} | ⏱️ {{fmtDuration .DurationMS}}{{end}}{{if .CostLinesAdded}} | {{fmtSigned .CostLinesAdded}}/{{fmtSigned .CostLinesRemoved}}{{end}}`
	engine, err := NewEngine(tmpl)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := StatusData{
		CostUSD:          0.05,
		DurationMS:       125000,
		CostLinesAdded:   42,
		CostLinesRemoved: -15,
	}

	result, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result, "💰 $0.05") {
		t.Errorf("Missing cost, got: %q", result)
	}
	if !strings.Contains(result, "⏱️ 2m 5s") {
		t.Errorf("Missing duration, got: %q", result)
	}
	if !strings.Contains(result, "+42/-15") {
		t.Errorf("Missing lines added/removed, got: %q", result)
	}
}
