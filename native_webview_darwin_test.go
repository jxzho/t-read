//go:build darwin

package main

import (
	"path/filepath"
	"testing"
)

func TestNativeWebviewRootDirWithEnv(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "nw")
	t.Setenv("TREAD_NATIVE_WEBVIEW_DIR", custom)

	got, err := nativeWebviewRootDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != custom {
		t.Fatalf("got %q, want %q", got, custom)
	}
}

func TestSHA256Hex(t *testing.T) {
	got := sha256Hex([]byte("abc"))
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
