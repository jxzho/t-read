//go:build darwin

package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func openNativeWebview(targetURL, theme string, skinScrollbar bool) error {
	if strings.TrimSpace(targetURL) == "" {
		return fmt.Errorf("empty target url")
	}

	helperPath, stateFilePath, err := ensureNativeWebviewHelper()
	if err != nil {
		return err
	}

	cmd := exec.Command(helperPath, targetURL, theme, strconv.FormatBool(skinScrollbar), stateFilePath)
	cmd.Env = append(os.Environ(), "TMPDIR=/tmp")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch native webview failed: %w", err)
	}

	return nil
}

func ensureNativeWebviewHelper() (helperPath string, stateFilePath string, err error) {
	rootDir, err := nativeWebviewRootDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve native webview dir failed: %w", err)
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create native webview dir failed: %w", err)
	}

	sourcePath := filepath.Join(rootDir, "native_webview_helper.swift")
	helperPath = filepath.Join(rootDir, "t-read")
	hashPath := filepath.Join(rootDir, "native_webview_helper.sha256")
	stateFilePath = filepath.Join(rootDir, "zoom.txt")

	wantHash := sha256Hex(nativeWebviewSwiftSource)
	currentHashBytes, readErr := os.ReadFile(hashPath)
	hasUpToDateHash := readErr == nil && strings.TrimSpace(string(currentHashBytes)) == wantHash
	if hasUpToDateHash {
		if info, statErr := os.Stat(helperPath); statErr == nil && !info.IsDir() {
			return helperPath, stateFilePath, nil
		}
	}

	if _, err := exec.LookPath("swiftc"); err != nil {
		return "", "", fmt.Errorf("swiftc not found: %w", err)
	}

	if err := os.WriteFile(sourcePath, nativeWebviewSwiftSource, 0o600); err != nil {
		return "", "", fmt.Errorf("write native helper source failed: %w", err)
	}

	compile := exec.Command("swiftc", "-O", sourcePath, "-o", helperPath)
	compile.Env = append(os.Environ(), "TMPDIR=/tmp")
	if output, err := compile.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("compile native helper failed: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	if err := os.WriteFile(hashPath, []byte(wantHash+"\n"), 0o644); err != nil {
		return "", "", fmt.Errorf("write helper hash failed: %w", err)
	}

	return helperPath, stateFilePath, nil
}

func nativeWebviewRootDir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("TREAD_NATIVE_WEBVIEW_DIR")); custom != "" {
		return custom, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".t-read", "native-webview"), nil
}

func sha256Hex(input []byte) string {
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

//go:embed assets/native_webview_helper.swift
var nativeWebviewSwiftSource []byte
