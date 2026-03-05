package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildReaderURLFromBookID(t *testing.T) {
	got, err := buildReaderURL("84132250538783841807d5c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://weread.qq.com/web/reader/84132250538783841807d5c"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildReaderURLFromURL(t *testing.T) {
	input := "https://weread.qq.com/web/reader/84132250538783841807d5c"
	got, err := buildReaderURL(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestBuildReaderURLRejectsInvalidBookID(t *testing.T) {
	if _, err := buildReaderURL("bad-id"); err == nil {
		t.Fatal("expected error for invalid id")
	}
}

func TestBuildReaderURLRejectsInvalidBookIDInReaderURL(t *testing.T) {
	input := "https://weread.qq.com/web/reader/not-valid"
	if _, err := buildReaderURL(input); err == nil {
		t.Fatal("expected error for invalid id in reader url")
	}
}

func TestNormalizeBookID(t *testing.T) {
	got, err := normalizeBookID("84132250538783841807D5C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "84132250538783841807d5c" {
		t.Fatalf("got %q", got)
	}
}

func TestBooksFilePathWithEnv(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "books.json")
	t.Setenv("TREAD_BOOKS_FILE", custom)

	got, err := booksFilePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != custom {
		t.Fatalf("got %q, want %q", got, custom)
	}
}

func TestLoadAndSaveBooks(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "books.json")
	t.Setenv("TREAD_BOOKS_FILE", custom)

	original := []book{{ID: "84132250538783841807d5c", Title: "Sample"}}
	if err := saveBooks(original); err != nil {
		t.Fatalf("saveBooks error: %v", err)
	}

	loaded, err := loadBooks()
	if err != nil {
		t.Fatalf("loadBooks error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 book, got %d", len(loaded))
	}
	if loaded[0].ID != original[0].ID || loaded[0].Title != original[0].Title {
		t.Fatalf("loaded book mismatch: %+v", loaded[0])
	}

	_, err = os.Stat(custom)
	if err != nil {
		t.Fatalf("expected books file created, stat error: %v", err)
	}
}

func TestParseReadOptionsDefault(t *testing.T) {
	opts, err := parseReadOptions([]string{"84132250538783841807d5c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.viewMode != viewModeBrowser {
		t.Fatalf("expected viewMode=%s, got=%s", viewModeBrowser, opts.viewMode)
	}
	if opts.theme != themeDark {
		t.Fatalf("expected theme=%s, got=%s", themeDark, opts.theme)
	}
	if !opts.skinScrollbar {
		t.Fatal("expected skinScrollbar=true")
	}
	if opts.input != "84132250538783841807d5c" {
		t.Fatalf("unexpected input: %q", opts.input)
	}
	if opts.legacyLaunch {
		t.Fatal("expected legacyLaunch=false")
	}
}

func TestParseReadOptionsWebview(t *testing.T) {
	opts, err := parseReadOptions([]string{"--webview", "84132250538783841807d5c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.viewMode != viewModeNative {
		t.Fatalf("expected viewMode=%s, got=%s", viewModeNative, opts.viewMode)
	}
	if opts.input != "84132250538783841807d5c" {
		t.Fatalf("unexpected input: %q", opts.input)
	}
	if !opts.legacyLaunch {
		t.Fatal("expected legacyLaunch=true")
	}
}

func TestParseReadOptionsNoInputWithTheme(t *testing.T) {
	opts, err := parseReadOptions([]string{"-w", "--theme", "light", "--no-scrollbar-skin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.viewMode != viewModeNative {
		t.Fatalf("expected viewMode=%s, got=%s", viewModeNative, opts.viewMode)
	}
	if opts.theme != themeLight {
		t.Fatalf("expected theme=%s, got=%s", themeLight, opts.theme)
	}
	if opts.skinScrollbar {
		t.Fatal("expected skinScrollbar=false")
	}
	if opts.input != "" {
		t.Fatalf("unexpected input: %q", opts.input)
	}
	if !opts.legacyLaunch {
		t.Fatal("expected legacyLaunch=true")
	}
}

func TestParseReadOptionsModeOverride(t *testing.T) {
	opts, err := parseReadOptions([]string{"--webview", "--webview-mode=app", "84132250538783841807d5c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.viewMode != viewModeApp {
		t.Fatalf("expected viewMode=%s, got=%s", viewModeApp, opts.viewMode)
	}
	if !opts.legacyLaunch {
		t.Fatal("expected legacyLaunch=true")
	}
}

func TestApplyUnifiedReadPolicyDarwin(t *testing.T) {
	opts := readOptions{
		viewMode:      viewModeBrowser,
		theme:         themeLight,
		skinScrollbar: false,
		input:         "84132250538783841807d5c",
		legacyLaunch:  true,
	}

	got := applyUnifiedReadPolicy(opts, "darwin")
	if got.viewMode != viewModeNative {
		t.Fatalf("expected viewMode=%s, got=%s", viewModeNative, got.viewMode)
	}
	if got.theme != themeDark {
		t.Fatalf("expected theme=%s, got=%s", themeDark, got.theme)
	}
	if !got.skinScrollbar {
		t.Fatal("expected skinScrollbar=true")
	}
	if got.input != opts.input {
		t.Fatalf("expected input=%q, got=%q", opts.input, got.input)
	}
	if !got.legacyLaunch {
		t.Fatal("expected legacyLaunch=true")
	}
}

func TestApplyUnifiedReadPolicyNonDarwin(t *testing.T) {
	opts := readOptions{
		viewMode:      viewModeNative,
		theme:         themeLight,
		skinScrollbar: false,
		input:         "84132250538783841807d5c",
	}

	got := applyUnifiedReadPolicy(opts, "linux")
	if got.viewMode != viewModeBrowser {
		t.Fatalf("expected viewMode=%s, got=%s", viewModeBrowser, got.viewMode)
	}
	if got.theme != themeLight {
		t.Fatalf("expected theme unchanged=%s, got=%s", themeLight, got.theme)
	}
	if got.skinScrollbar {
		t.Fatal("expected skinScrollbar unchanged=false")
	}
}

func TestParseReadOptionsRejectsTooManyArgs(t *testing.T) {
	if _, err := parseReadOptions([]string{"a", "b"}); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseReadOptionsRejectsInvalidMode(t *testing.T) {
	if _, err := parseReadOptions([]string{"--webview-mode", "invalid"}); err == nil {
		t.Fatal("expected parse error for invalid mode")
	}
}

func TestParseReadOptionsRejectsInvalidTheme(t *testing.T) {
	if _, err := parseReadOptions([]string{"--theme", "sepia"}); err == nil {
		t.Fatal("expected parse error for invalid theme")
	}
}

func TestWebviewLaunchCandidates(t *testing.T) {
	got := webviewLaunchCandidates("https://weread.qq.com/web/reader/84132250538783841807d5c")
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		if len(got) == 0 {
			t.Fatal("expected candidates for supported os")
		}
	default:
		if got != nil {
			t.Fatal("expected nil for unsupported os")
		}
	}
}
