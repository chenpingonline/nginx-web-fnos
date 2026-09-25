// A full-ports-only lifecycle helper. Never follows links into external data.
package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

func repair(parent int, name string, uid, gid int, device uint64) error {
	fd, err := unix.Openat(parent, name, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err == unix.ELOOP {
		return nil
	} // External symlinks are not application data.
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err != nil {
		return err
	}
	if device != 0 && st.Dev != device {
		return fmt.Errorf("refusing mounted directory: %s", name)
	}
	kind := st.Mode & unix.S_IFMT
	if kind != unix.S_IFDIR && kind != unix.S_IFREG {
		return nil
	}
	if kind == unix.S_IFREG && st.Nlink != 1 {
		return fmt.Errorf("refusing hard-linked file: %s", name)
	}
	// Reopen the pinned inode, not the user-controlled name (also avoids opening devices).
	rwfd, err := unix.Open(fmt.Sprintf("/proc/self/fd/%d", fd), unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	data := os.NewFile(uintptr(rwfd), name)
	defer data.Close()
	fd = rwfd
	// Work through pinned descriptors, never resolve a child path again for writes.
	if err = unix.Fchown(fd, uid, gid); err != nil {
		return err
	}
	mode := st.Mode&0777 | 0600
	if kind == unix.S_IFDIR {
		mode |= 0100
	}
	if err = unix.Fchmod(fd, mode); err != nil {
		return err
	}
	if kind == unix.S_IFDIR {
		names, err := data.Readdirnames(-1)
		if err != nil {
			return err
		}
		for _, child := range names {
			if err := repair(fd, child, uid, gid, st.Dev); err != nil {
				return fmt.Errorf("%s: %w", child, err)
			}
		}
	}
	return nil
}

func run() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run in privileged installation stage")
	}
	account, err := user.Lookup("nginx-web")
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil || uid == 0 {
		return fmt.Errorf("invalid application uid")
	}
	group, err := user.LookupGroup("nginx-web")
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return err
	}
	for _, key := range []string{"TRIM_PKGETC", "TRIM_PKGVAR", "TRIM_PKGHOME"} {
		path := os.Getenv(key)
		if !filepath.IsAbs(path) {
			return fmt.Errorf("invalid %s", key)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		switch path {
		case "/", "/etc", "/var", "/tmp", "/home", "/usr", "/opt":
			return fmt.Errorf("refusing system directory %s", path)
		}
		if err := repair(unix.AT_FDCWD, path, uid, gid, 0); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "应用数据权限修复失败：", err)
		os.Exit(1)
	}
}
