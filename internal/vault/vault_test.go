package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func testVault(t *testing.T) *Vault {
	t.Helper()
	v, err := Load(filepath.Join(t.TempDir(), "vault.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return v
}

func TestSetupUnlockAndSecrets(t *testing.T) {
	v := testVault(t)
	if v.Initialized() {
		t.Fatal("new vault should not be initialized")
	}
	if err := v.Unlock("hunter2"); err != ErrNotInitialized {
		t.Fatalf("expected ErrNotInitialized, got %v", err)
	}
	if err := v.Setup("correct horse battery staple"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !v.Initialized() {
		t.Fatal("vault should be initialized after setup")
	}
	if !v.Unlocked() {
		t.Fatal("vault should be unlocked after setup")
	}

	e := Entry{Type: EntryPassword, Name: "Email", Username: "me@example.com"}
	if err := v.AddEntry(e, "s3cret!"); err != nil {
		t.Fatalf("add entry: %v", err)
	}
	entries := v.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Group != DefaultGroup {
		t.Fatalf("expected default group %q, got %q", DefaultGroup, entries[0].Group)
	}
	if entries[0].Secret == "s3cret!" {
		t.Fatal("secret stored in plaintext")
	}

	plain, err := v.RevealSecret(entries[0].ID)
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if plain != "s3cret!" {
		t.Fatalf("expected s3cret!, got %q", plain)
	}

	reloaded, err := Load(v.path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if err := reloaded.Unlock("wrong password"); err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
	if err := reloaded.Unlock("correct horse battery staple"); err != nil {
		t.Fatalf("unlock with correct password: %v", err)
	}
	plain, err = reloaded.RevealSecret(entries[0].ID)
	if err != nil {
		t.Fatalf("reveal after reload: %v", err)
	}
	if plain != "s3cret!" {
		t.Fatalf("expected s3cret! after reload, got %q", plain)
	}
}

func TestUpdateAndDelete(t *testing.T) {
	v := testVault(t)
	if err := v.Setup("masterpass123"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	e := Entry{Type: EntryAPIKey, Name: "Stripe", Group: "Work"}
	if err := v.AddEntry(e, "sk_live_123"); err != nil {
		t.Fatalf("add: %v", err)
	}
	id := v.Entries()[0].ID

	updated := Entry{Type: EntryAPIKey, Name: "Stripe Prod", Group: "Work"}
	if err := v.UpdateEntry(id, updated, ""); err != nil {
		t.Fatalf("update without secret: %v", err)
	}
	plain, _ := v.RevealSecret(id)
	if plain != "sk_live_123" {
		t.Fatal("secret should be preserved when update secret is empty")
	}

	if err := v.UpdateEntry(id, updated, "sk_live_456"); err != nil {
		t.Fatalf("update with secret: %v", err)
	}
	plain, _ = v.RevealSecret(id)
	if plain != "sk_live_456" {
		t.Fatalf("expected rotated secret, got %q", plain)
	}

	if err := v.DeleteEntry(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := v.Get(id); err != ErrEntryNotFound {
		t.Fatalf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := randomBytes(32)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := encrypt(key, []byte("top secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	plain, err := decrypt(key, sealed)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plain) != "top secret" {
		t.Fatalf("round trip mismatch: %q", plain)
	}
	other, _ := randomBytes(32)
	if _, err := decrypt(other, sealed); err != ErrDecryptionFailed {
		t.Fatalf("expected ErrDecryptionFailed with wrong key, got %v", err)
	}
}

func TestLockClearsKey(t *testing.T) {
	v := testVault(t)
	if err := v.Setup("masterpass123"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	v.Lock()
	if v.Unlocked() {
		t.Fatal("vault should be locked")
	}
	err := v.AddEntry(Entry{Name: "x"}, "y")
	if err != ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
}

func TestSaveEnforcesPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	v, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Setup("masterpass123"); err != nil {
		t.Fatal(err)
	}

	// loosen perms as if the user had chmod'ed things
	os.Chmod(path, 0o644)
	os.Chmod(dir, 0o755)

	if err := v.AddEntry(Entry{Type: EntryPassword, Name: "x"}, "y"); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("vault file perm = %o, want 600", perm)
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Fatalf("vault dir perm = %o, want 700", perm)
	}
}
