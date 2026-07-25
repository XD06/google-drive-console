package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	content := "FOO_TEST_A=one\n# comment\nFOO_TEST_B=\"two\"\nexport FOO_TEST_C=three\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("FOO_TEST_A")
	_ = os.Unsetenv("FOO_TEST_B")
	_ = os.Unsetenv("FOO_TEST_C")
	os.Setenv("FOO_TEST_B", "keep") // should not overwrite
	if err := LoadDotEnv(p); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FOO_TEST_A") != "one" {
		t.Fatalf("A=%q", os.Getenv("FOO_TEST_A"))
	}
	if os.Getenv("FOO_TEST_B") != "keep" {
		t.Fatalf("B overwritten")
	}
	if os.Getenv("FOO_TEST_C") != "three" {
		t.Fatalf("C=%q", os.Getenv("FOO_TEST_C"))
	}
	if err := LoadDotEnv(filepath.Join(dir, "missing.env")); err != nil {
		t.Fatal(err)
	}
}