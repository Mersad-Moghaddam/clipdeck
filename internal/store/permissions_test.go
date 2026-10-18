package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRestrictsExistingPrivateDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "blobs"), filepath.Join(dir, "thumbs")} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, path := range []string{dir, filepath.Join(dir, "blobs"), filepath.Join(dir, "thumbs")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0700 {
			t.Errorf("%s permissions = %o, want 700", path, info.Mode().Perm())
		}
	}
}

func TestOpenRejectsSymlinkPrivateDirectory(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Chmod(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "blobs")); err != nil {
		t.Fatal(err)
	}
	s, err := Open(root)
	if err == nil {
		s.Close()
		t.Fatal("accepted symlink blob directory")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatal("changed symlink target permissions")
	}
}
