package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const readerBaseURL = "https://weread.qq.com/web/reader/"

const (
	viewModeBrowser = "browser"
	viewModeApp     = "app"
	viewModeNative  = "native"

	themeDark  = "dark"
	themeLight = "light"
)

var bookIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{23,24}$`)

type book struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type readOptions struct {
	viewMode      string
	theme         string
	skinScrollbar bool
	input         string
	legacyLaunch  bool
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "read":
		if err := runRead(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "read failed: %v\n", err)
			os.Exit(1)
		}
	case "book":
		if err := runBook(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "book command failed: %v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func runRead(args []string) error {
	opts, err := parseReadOptions(args)
	if err != nil {
		return err
	}
	if opts.legacyLaunch {
		fmt.Fprintln(os.Stderr, "legacy launch flags are accepted but ignored by unified launch policy")
	}
	opts = applyUnifiedReadPolicy(opts, runtime.GOOS)

	if opts.input == "" {
		books, err := loadBooks()
		if err != nil {
			return err
		}
		if len(books) == 0 {
			return errors.New("no saved books; add one with: t-read book add <bookId> <title>")
		}

		selected, err := chooseBookInteractively(books)
		if err != nil {
			return err
		}

		targetURL, err := buildReaderURL(selected.ID)
		if err != nil {
			return err
		}
		modeUsed, err := openReader(targetURL, opts)
		if err != nil {
			return err
		}

		fmt.Printf("opened: %s (%s) [mode=%s]\n", selected.Title, targetURL, modeUsed)
		return nil
	}

	targetURL, err := buildReaderURL(opts.input)
	if err != nil {
		return err
	}

	modeUsed, err := openReader(targetURL, opts)
	if err != nil {
		return err
	}

	fmt.Printf("opened: %s [mode=%s]\n", targetURL, modeUsed)
	return nil
}

func runBook(args []string) error {
	if len(args) == 0 {
		return errors.New("missing subcommand; use: t-read book <add|list>")
	}

	switch args[0] {
	case "add":
		if len(args) < 3 {
			return errors.New("usage: t-read book add <bookId> <title>")
		}

		id, err := normalizeBookID(args[1])
		if err != nil {
			return err
		}
		title := strings.TrimSpace(strings.Join(args[2:], " "))
		if title == "" {
			return errors.New("title cannot be empty")
		}

		books, err := loadBooks()
		if err != nil {
			return err
		}

		updated := false
		for i := range books {
			if books[i].ID == id {
				books[i].Title = title
				updated = true
				break
			}
		}
		if !updated {
			books = append(books, book{ID: id, Title: title})
		}

		sort.Slice(books, func(i, j int) bool {
			return strings.ToLower(books[i].Title) < strings.ToLower(books[j].Title)
		})

		if err := saveBooks(books); err != nil {
			return err
		}

		if updated {
			fmt.Printf("updated: [%s] %s\n", id, title)
		} else {
			fmt.Printf("added: [%s] %s\n", id, title)
		}
		return nil

	case "list":
		books, err := loadBooks()
		if err != nil {
			return err
		}

		if len(books) == 0 {
			fmt.Println("no saved books")
			return nil
		}

		for i, b := range books {
			fmt.Printf("%d. %s (%s)\n", i+1, b.Title, b.ID)
		}
		return nil
	default:
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

func buildReaderURL(input string) (string, error) {
	if input == "" {
		return "", errors.New("empty bookId or url")
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		u, err := url.Parse(input)
		if err != nil {
			return "", err
		}
		if u.Host == "" {
			return "", errors.New("url missing host")
		}
		id, err := extractBookIDFromURL(u)
		if err == nil {
			_, err = normalizeBookID(id)
			if err != nil {
				return "", err
			}
		}
		return u.String(), nil
	}

	id, err := normalizeBookID(input)
	if err != nil {
		return "", err
	}

	return readerBaseURL + id, nil
}

func extractBookIDFromURL(u *url.URL) (string, error) {
	if u.Host != "weread.qq.com" {
		return "", errors.New("unsupported host")
	}

	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "web" && parts[1] == "reader" {
		return parts[2], nil
	}

	return "", errors.New("not a reader url")
}

func normalizeBookID(input string) (string, error) {
	id := strings.TrimSpace(input)
	if !bookIDPattern.MatchString(id) {
		return "", errors.New("bookId must be 23-24 hex characters, e.g. 84132250538783841807d5c")
	}
	return strings.ToLower(id), nil
}

func loadBooks() ([]book, error) {
	filePath, err := booksFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []book{}, nil
		}
		return nil, err
	}

	var books []book
	if err := json.Unmarshal(data, &books); err != nil {
		return nil, fmt.Errorf("invalid books file format: %w", err)
	}
	return books, nil
}

func saveBooks(books []book) error {
	filePath, err := booksFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(books, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filePath, data, 0o644)
}

func booksFilePath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("TREAD_BOOKS_FILE")); custom != "" {
		return custom, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".t-read", "books.json"), nil
}

func chooseBookInteractively(books []book) (book, error) {
	fmt.Println("Choose a book:")
	for i, b := range books {
		fmt.Printf("%d. %s (%s)\n", i+1, b.Title, b.ID)
	}
	fmt.Print("Enter number (or q to quit): ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return book{}, err
	}
	line = strings.TrimSpace(line)
	if strings.EqualFold(line, "q") {
		return book{}, errors.New("cancelled")
	}

	n, err := strconv.Atoi(line)
	if err != nil {
		return book{}, errors.New("invalid input, expected number")
	}
	if n < 1 || n > len(books) {
		return book{}, errors.New("selection out of range")
	}
	return books[n-1], nil
}

func parseReadOptions(args []string) (readOptions, error) {
	opts := readOptions{
		viewMode:      viewModeBrowser,
		theme:         themeDark,
		skinScrollbar: true,
	}
	remaining := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--webview", "-w":
			opts.legacyLaunch = true
			opts.viewMode = viewModeNative
		case "--no-scrollbar-skin":
			opts.legacyLaunch = true
			opts.skinScrollbar = false
		case "--webview-mode":
			opts.legacyLaunch = true
			i++
			if i >= len(args) {
				return readOptions{}, errors.New("missing value for --webview-mode")
			}
			mode, err := normalizeViewMode(args[i])
			if err != nil {
				return readOptions{}, err
			}
			opts.viewMode = mode
		case "--theme":
			opts.legacyLaunch = true
			i++
			if i >= len(args) {
				return readOptions{}, errors.New("missing value for --theme")
			}
			theme, err := normalizeTheme(args[i])
			if err != nil {
				return readOptions{}, err
			}
			opts.theme = theme
		default:
			if strings.HasPrefix(a, "--webview-mode=") {
				opts.legacyLaunch = true
				mode, err := normalizeViewMode(strings.TrimPrefix(a, "--webview-mode="))
				if err != nil {
					return readOptions{}, err
				}
				opts.viewMode = mode
				continue
			}
			if strings.HasPrefix(a, "--theme=") {
				opts.legacyLaunch = true
				theme, err := normalizeTheme(strings.TrimPrefix(a, "--theme="))
				if err != nil {
					return readOptions{}, err
				}
				opts.theme = theme
				continue
			}
			remaining = append(remaining, a)
		}
	}

	if len(remaining) > 1 {
		return readOptions{}, errors.New("usage: t-read read [bookId|url] (legacy launch flags are accepted but ignored)")
	}
	if len(remaining) == 1 {
		opts.input = strings.TrimSpace(remaining[0])
	}

	return opts, nil
}

func applyUnifiedReadPolicy(opts readOptions, goos string) readOptions {
	unified := opts
	switch goos {
	case "darwin":
		unified.viewMode = viewModeNative
		unified.theme = themeDark
		unified.skinScrollbar = true
	default:
		unified.viewMode = viewModeBrowser
	}
	return unified
}

func normalizeViewMode(mode string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(mode))
	switch value {
	case viewModeNative, viewModeApp, viewModeBrowser:
		return value, nil
	default:
		return "", fmt.Errorf("invalid --webview-mode: %s (expected native|app|browser)", mode)
	}
}

func normalizeTheme(theme string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(theme))
	switch value {
	case themeDark, themeLight:
		return value, nil
	default:
		return "", fmt.Errorf("invalid --theme: %s (expected dark|light)", theme)
	}
}

func openReader(targetURL string, opts readOptions) (string, error) {
	switch opts.viewMode {
	case viewModeBrowser:
		return viewModeBrowser, openBrowser(targetURL)
	case viewModeApp:
		if err := openWebContainer(targetURL); err != nil {
			fmt.Fprintf(os.Stderr, "app container unavailable (%v), fallback to default browser\n", err)
			return viewModeBrowser, openBrowser(targetURL)
		}
		return viewModeApp, nil
	case viewModeNative:
		if err := openNativeWebview(targetURL, opts.theme, opts.skinScrollbar); err != nil {
			fmt.Fprintf(os.Stderr, "native webview unavailable (%v), fallback to app container\n", err)
			if appErr := openWebContainer(targetURL); appErr != nil {
				fmt.Fprintf(os.Stderr, "app container unavailable (%v), fallback to default browser\n", appErr)
				return viewModeBrowser, openBrowser(targetURL)
			}
			return viewModeApp, nil
		}
		return viewModeNative, nil
	default:
		return "", fmt.Errorf("unsupported view mode: %s", opts.viewMode)

	}
}

func openWebContainer(targetURL string) error {
	candidates := webviewLaunchCandidates(targetURL)
	if len(candidates) == 0 {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	var lastErr error
	for _, c := range candidates {
		if len(c) == 0 {
			continue
		}
		cmd := exec.Command(c[0], c[1:]...)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("no available webview launcher")
}

func webviewLaunchCandidates(targetURL string) [][]string {
	switch runtime.GOOS {
	case "darwin":
		return [][]string{
			{"open", "-na", "Google Chrome", "--args", "--app=" + targetURL},
			{"open", "-na", "Microsoft Edge", "--args", "--app=" + targetURL},
			{"open", "-na", "Brave Browser", "--args", "--app=" + targetURL},
			{"open", "-na", "Chromium", "--args", "--app=" + targetURL},
		}
	case "linux":
		return [][]string{
			{"google-chrome", "--app=" + targetURL},
			{"microsoft-edge", "--app=" + targetURL},
			{"brave-browser", "--app=" + targetURL},
			{"chromium", "--app=" + targetURL},
			{"chromium-browser", "--app=" + targetURL},
		}
	case "windows":
		return [][]string{
			{"cmd", "/c", "start", "msedge", "--app=" + targetURL},
			{"cmd", "/c", "start", "chrome", "--app=" + targetURL},
		}
	default:
		return nil
	}
}

func openBrowser(targetURL string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "linux":
		cmd = exec.Command("xdg-open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return cmd.Start()
}

func printUsage() {
	fmt.Println("t-read: open WeRead books from terminal")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  t-read read <bookId|url>")
	fmt.Println("  t-read read")
	fmt.Println("  t-read book add <bookId> <title>")
	fmt.Println("  t-read book list")
	fmt.Println("")
	fmt.Println("Unified launch policy:")
	fmt.Println("  macOS: native webview in dark mode (fallback: app container -> browser)")
	fmt.Println("  non-macOS: default browser")
	fmt.Println("  legacy flags accepted but ignored: --webview|-w --webview-mode --theme --no-scrollbar-skin")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  t-read read 84132250538783841807d5c")
	fmt.Println("  t-read read --theme light 84132250538783841807d5c  # accepted, but ignored")
	fmt.Println("  t-read read https://weread.qq.com/web/reader/84132250538783841807d5c")
	fmt.Println("  t-read book add 84132250538783841807d5c Three Body Problem")
	fmt.Println("  t-read read")
	fmt.Println("")
	fmt.Println("Native webview shortcuts (macOS):")
	fmt.Println("  Cmd + '+' : zoom in")
	fmt.Println("  Cmd + '-' : zoom out")
	fmt.Println("  Cmd + '0' : reset zoom")
	fmt.Println("  (zoom level is saved and reused)")
}
