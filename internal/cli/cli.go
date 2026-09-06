package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"lockerapp/internal/vault"
)

// Add inserts an entry into the vault without launching the TUI.
// The master password and the secret are always read from stdin (one per
// line) when piped, or prompted for with echo disabled on a terminal, so
// they never appear in argv or shell history.
func Add(args []string, stdin io.Reader) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	typ := fs.String("type", "apikey", "entry type: password or apikey")
	name := fs.String("name", "", "entry name (required)")
	group := fs.String("group", "", "group (default: "+vault.DefaultGroup+")")
	username := fs.String("username", "", "username or key id (optional)")
	notes := fs.String("notes", "", "notes (optional)")
	vaultPath := fs.String("vault", "", "vault file path (default: user config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("--name is required")
	}

	var entryType vault.EntryType
	switch vault.EntryType(*typ) {
	case vault.EntryPassword, vault.EntryAPIKey:
		entryType = vault.EntryType(*typ)
	default:
		return fmt.Errorf("invalid --type %q (want password or apikey)", *typ)
	}

	path := *vaultPath
	if path == "" {
		var err error
		path, err = vault.DefaultPath()
		if err != nil {
			return err
		}
	}
	v, err := vault.Load(path)
	if err != nil {
		return err
	}
	if !v.Initialized() {
		return errors.New("vault not initialized; run 'locker' first to create a master password")
	}

	sr := newSecretReader(stdin)
	password, err := sr.read("Master password: ")
	if err != nil {
		return fmt.Errorf("reading master password: %w", err)
	}
	if err := v.Unlock(password); err != nil {
		return errors.New("incorrect master password")
	}
	defer v.Lock()

	sec, err := sr.read("Secret: ")
	if err != nil {
		return fmt.Errorf("reading secret: %w", err)
	}
	if sec == "" {
		return errors.New("secret is required")
	}

	e := vault.Entry{
		Type:     entryType,
		Name:     *name,
		Group:    *group,
		Username: *username,
		Notes:    *notes,
	}
	if err := v.AddEntry(e, sec); err != nil {
		return err
	}
	g := e.Group
	if g == "" {
		g = vault.DefaultGroup
	}
	fmt.Printf("added %s %q to group %q\n", entryType, e.Name, g)
	return nil
}

// secretReader reads sensitive lines either from a terminal with echo
// disabled or line-by-line from a pipe, keeping a shared buffered reader so
// consecutive reads do not lose buffered input.
type secretReader struct {
	tty bool
	fd  int
	r   *bufio.Reader
}

func newSecretReader(stdin io.Reader) *secretReader {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return &secretReader{tty: true, fd: int(f.Fd())}
	}
	return &secretReader{r: bufio.NewReader(stdin)}
}

func (s *secretReader) read(prompt string) (string, error) {
	if s.tty {
		fmt.Fprint(os.Stderr, prompt)
		b, err := term.ReadPassword(s.fd)
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	line, err := s.r.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil && line == "" {
		return "", err
	}
	return line, nil
}
