package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"lockerapp/internal/vault"
)

func TestAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	if err := v.Setup("testmaster123"); err != nil {
		t.Fatal(err)
	}
	v.Lock()

	args := []string{
		"--vault", path,
		"--type", "apikey",
		"--name", "Stripe",
		"--group", "Payments",
		"--username", "prod",
	}
	if err := Add(args, strings.NewReader("wrongpassword\nsk_live_abc\n")); err == nil {
		t.Fatal("expected error with wrong password")
	}
	if err := Add(args, strings.NewReader("testmaster123\nsk_live_abc\n")); err != nil {
		t.Fatalf("add: %v", err)
	}

	reloaded, _ := vault.Load(path)
	if err := reloaded.Unlock("testmaster123"); err != nil {
		t.Fatal(err)
	}
	entries := reloaded.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Name != "Stripe" || e.Group != "Payments" || e.Type != vault.EntryAPIKey {
		t.Fatalf("unexpected entry: %+v", e)
	}
	s, err := reloaded.RevealSecret(e.ID)
	if err != nil || s != "sk_live_abc" {
		t.Fatalf("reveal: %v %q", err, s)
	}
}

func TestAddValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	v.Setup("testmaster123")

	if err := Add([]string{"--vault", path}, strings.NewReader("pw\n")); err == nil {
		t.Fatal("expected error for missing name")
	}
	if err := Add([]string{"--vault", path, "--name", "x"}, strings.NewReader("testmaster123\n\n")); err == nil {
		t.Fatal("expected error for missing secret")
	}
	if err := Add([]string{"--vault", path, "--type", "bogus", "--name", "x"}, strings.NewReader("pw\n")); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestAddSecretFromStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	if err := v.Setup("testmaster123"); err != nil {
		t.Fatal(err)
	}
	v.Lock()

	// first stdin line is the master password, second is the secret
	args := []string{"--vault", path, "--type", "apikey", "--name", "HiddenSecret"}
	if err := Add(args, strings.NewReader("testmaster123\nsk_hidden_999\n")); err != nil {
		t.Fatalf("add: %v", err)
	}

	reloaded, _ := vault.Load(path)
	reloaded.Unlock("testmaster123")
	entries := reloaded.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	s, _ := reloaded.RevealSecret(entries[0].ID)
	if s != "sk_hidden_999" {
		t.Fatalf("expected sk_hidden_999, got %q", s)
	}
}

func TestAddEmptySecretRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	v, _ := vault.Load(path)
	v.Setup("testmaster123")
	v.Lock()

	args := []string{"--vault", path, "--name", "Empty"}
	if err := Add(args, strings.NewReader("testmaster123\n\n")); err == nil {
		t.Fatal("expected error for empty secret")
	}
}
