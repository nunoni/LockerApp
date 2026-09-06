package vault

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type EntryType string

const (
	EntryPassword EntryType = "password"
	EntryAPIKey   EntryType = "apikey"
)

const DefaultGroup = "General"

type Entry struct {
	ID        string    `json:"id"`
	Type      EntryType `json:"type"`
	Name      string    `json:"name"`
	Group     string    `json:"group"`
	Username  string    `json:"username,omitempty"`
	Secret    string    `json:"secret"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type fileFormat struct {
	Version    int     `json:"version"`
	AuthSalt   string  `json:"auth_salt"`
	MasterHash string  `json:"master_hash"`
	EncSalt    string  `json:"enc_salt"`
	Entries    []Entry `json:"entries"`
}

type Vault struct {
	path string
	data fileFormat
	key  []byte
}

var (
	ErrNotInitialized = errors.New("vault is not initialized")
	ErrWrongPassword  = errors.New("incorrect master password")
	ErrLocked         = errors.New("vault is locked")
	ErrEntryNotFound  = errors.New("entry not found")
)

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config dir: %w", err)
	}
	return filepath.Join(dir, "locker", "vault.json"), nil
}

func Load(path string) (*Vault, error) {
	v := &Vault{path: path}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading vault: %w", err)
	}
	if err := json.Unmarshal(raw, &v.data); err != nil {
		return nil, fmt.Errorf("parsing vault: %w", err)
	}
	if v.data.Version != 1 {
		return nil, fmt.Errorf("unsupported vault version %d", v.data.Version)
	}
	return v, nil
}

func (v *Vault) Initialized() bool {
	return v.data.MasterHash != ""
}

func (v *Vault) Unlocked() bool {
	return v.key != nil
}

func (v *Vault) Setup(masterPassword string) error {
	if v.Initialized() {
		return errors.New("vault already initialized")
	}
	hash, authSalt, err := hashMasterPassword(masterPassword)
	if err != nil {
		return err
	}
	encSaltRaw, err := randomBytes(saltLen)
	if err != nil {
		return err
	}
	v.data = fileFormat{
		Version:    1,
		AuthSalt:   authSalt,
		MasterHash: hash,
		EncSalt:    base64.StdEncoding.EncodeToString(encSaltRaw),
		Entries:    []Entry{},
	}
	v.key = deriveKey(masterPassword, encSaltRaw)
	return v.save()
}

func (v *Vault) Unlock(masterPassword string) error {
	if !v.Initialized() {
		return ErrNotInitialized
	}
	if !verifyMasterPassword(masterPassword, v.data.MasterHash, v.data.AuthSalt) {
		return ErrWrongPassword
	}
	encSalt, err := base64.StdEncoding.DecodeString(v.data.EncSalt)
	if err != nil {
		return fmt.Errorf("decoding encryption salt: %w", err)
	}
	v.key = deriveKey(masterPassword, encSalt)
	return nil
}

func (v *Vault) Lock() {
	if v.key != nil {
		for i := range v.key {
			v.key[i] = 0
		}
		v.key = nil
	}
}

func (v *Vault) Entries() []Entry {
	entries := make([]Entry, len(v.data.Entries))
	copy(entries, v.data.Entries)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Group != entries[j].Group {
			return entries[i].Group < entries[j].Group
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func (v *Vault) Get(id string) (Entry, error) {
	for _, e := range v.data.Entries {
		if e.ID == id {
			return e, nil
		}
	}
	return Entry{}, ErrEntryNotFound
}

func newID() (string, error) {
	b, err := randomBytes(8)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (v *Vault) AddEntry(e Entry, secret string) error {
	if !v.Unlocked() {
		return ErrLocked
	}
	id, err := newID()
	if err != nil {
		return err
	}
	sealed, err := encrypt(v.key, []byte(secret))
	if err != nil {
		return err
	}
	if e.Group == "" {
		e.Group = DefaultGroup
	}
	e.ID = id
	e.Secret = sealed
	e.CreatedAt = time.Now()
	e.UpdatedAt = e.CreatedAt
	v.data.Entries = append(v.data.Entries, e)
	return v.save()
}

func (v *Vault) UpdateEntry(id string, e Entry, secret string) error {
	if !v.Unlocked() {
		return ErrLocked
	}
	for i := range v.data.Entries {
		if v.data.Entries[i].ID == id {
			if e.Group == "" {
				e.Group = DefaultGroup
			}
			e.ID = id
			e.CreatedAt = v.data.Entries[i].CreatedAt
			e.UpdatedAt = time.Now()
			if secret != "" {
				sealed, err := encrypt(v.key, []byte(secret))
				if err != nil {
					return err
				}
				e.Secret = sealed
			} else {
				e.Secret = v.data.Entries[i].Secret
			}
			v.data.Entries[i] = e
			return v.save()
		}
	}
	return ErrEntryNotFound
}

func (v *Vault) DeleteEntry(id string) error {
	if !v.Unlocked() {
		return ErrLocked
	}
	for i := range v.data.Entries {
		if v.data.Entries[i].ID == id {
			v.data.Entries = append(v.data.Entries[:i], v.data.Entries[i+1:]...)
			return v.save()
		}
	}
	return ErrEntryNotFound
}

func (v *Vault) RevealSecret(id string) (string, error) {
	if !v.Unlocked() {
		return "", ErrLocked
	}
	e, err := v.Get(id)
	if err != nil {
		return "", err
	}
	plain, err := decrypt(v.key, e.Secret)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (v *Vault) save() error {
	dir := filepath.Dir(v.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating vault dir: %w", err)
	}
	// MkdirAll is a no-op for existing dirs, so enforce perms explicitly.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("securing vault dir: %w", err)
	}
	return withVaultLock(v.path, func() error {
		raw, err := json.MarshalIndent(v.data, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding vault: %w", err)
		}
		tmp, err := os.CreateTemp(dir, ".locker-vault-*.tmp")
		if err != nil {
			return fmt.Errorf("creating temp vault: %w", err)
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName) // no-op after a successful rename
		// CreateTemp already uses 0600, but enforce it explicitly anyway.
		if err := tmp.Chmod(0o600); err != nil {
			tmp.Close()
			return fmt.Errorf("securing vault file: %w", err)
		}
		if _, err := tmp.Write(raw); err != nil {
			tmp.Close()
			return fmt.Errorf("writing vault: %w", err)
		}
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			return fmt.Errorf("syncing vault: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("closing vault: %w", err)
		}
		if err := os.Rename(tmpName, v.path); err != nil {
			return fmt.Errorf("replacing vault: %w", err)
		}
		// Best-effort: persist the directory entry so a crash right after
		// the rename cannot lose the new vault file.
		if d, err := os.Open(dir); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
		return nil
	})
}

// withVaultLock serialises writes from concurrent processes. The lock is held
// only for the duration of a single write, which keeps each write atomic and
// uncorrupted. It does not serialise a full read-modify-write cycle, so two
// processes editing simultaneously can still race logically (last writer wins).
func withVaultLock(path string, fn func() error) error {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening vault lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("vault is in use by another process")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}
