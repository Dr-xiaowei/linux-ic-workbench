package workbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// The platform database is personal even when projects and the installation are shared.
// Refuse unsafe existing permissions instead of changing a shared directory's access.
func PrivateDir(path string, create bool) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.MkdirAll(abs, 0700); err != nil {
			return "", err
		}
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || !owned(info) {
		return "", fmt.Errorf("private directory must be owned by UID %d, a real directory and mode 0700: %s", os.Geteuid(), abs)
	}
	// Resolve ancestors and reject writable parents that allow another user to replace it.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	unsafeParent := ""
	for p := filepath.Dir(real); ; p = filepath.Dir(p) {
		st, e := os.Stat(p)
		if e != nil {
			return "", e
		}
		if owned(st) && st.Mode().Perm()&0077 == 0 {
			unsafeParent = "" // An enclosing private home prevents cross-user traversal.
			break
		}
		if st.Mode().Perm()&0022 != 0 && st.Mode()&os.ModeSticky == 0 {
			unsafeParent = p
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if unsafeParent != "" {
		return "", fmt.Errorf("private directory has a group/other-writable parent: %s", unsafeParent)
	}
	return real, nil
}

func owned(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid())
}

func PrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || !owned(info) || info.Mode().Perm()&0077 != 0 || !ok || st.Nlink != 1 {
		return fmt.Errorf("private file must be owned by UID %d, mode 0600 and not a link: %s", os.Geteuid(), path)
	}
	return nil
}

func privateLock(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	if err = PrivateFile(path); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// Config templates are read-only distribution files. Lock and atomic publication
// ensure simultaneous first launches never truncate a personal configuration.
func LoadConfig(path, template string) (Config, error) {
	c := Defaults()
	if _, err := PrivateDir(filepath.Dir(path), template != ""); err != nil {
		return c, err
	}
	lock, err := privateLock(filepath.Join(filepath.Dir(path), ".config.lock"))
	if err != nil {
		return c, err
	}
	defer lock.Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			return c, fmt.Errorf("configuration busy: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if _, err := os.Lstat(path); os.IsNotExist(err) && template != "" {
		b, err := readBounded(template, 1<<20)
		if err != nil {
			return c, err
		}
		if _, err = decodeConfig(b); err != nil {
			return c, err
		}
		if err = atomicFile(path, b); err != nil {
			return c, err
		}
	}
	if err := PrivateFile(path); err != nil {
		return c, err
	}
	b, err := readBounded(path, 1<<20)
	if err != nil {
		return c, err
	}
	return decodeConfig(b)
}

func decodeConfig(b []byte) (Config, error) {
	c := Defaults()
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return c, errors.New("configuration requires a JSON object")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return c, errors.New("configuration requires a single JSON object")
	}
	return c, nil
}
