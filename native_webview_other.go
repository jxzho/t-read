//go:build !darwin

package main

import "fmt"

func openNativeWebview(targetURL, theme string, skinScrollbar bool) error {
	_ = targetURL
	_ = theme
	_ = skinScrollbar
	return fmt.Errorf("native webview is only supported on macOS")
}
