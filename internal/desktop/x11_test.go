package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestX11ReadUTF8(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$*\" in *TARGETS*) printf 'UTF8_STRING\\ntext/plain\\n';; *) printf 'hello X11\\n';; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	got, err := NewX11Backend().Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.MIME != "text/plain;charset=utf-8" || string(got.Data) != "hello X11\n" {
		t.Fatalf("selection=%+v", got)
	}
}

func TestX11ReadSecretSelection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xclip"), []byte("#!/bin/sh\nprintf 'UTF8_STRING\\napplication/x-secret-flag\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	got, err := NewX11Backend().Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 0 {
		t.Fatal("secret clipboard retained")
	}
}

func TestX11PasteRejectsInvalidTarget(t *testing.T) {
	if err := NewX11Backend().Paste(context.Background(), Window{ID: "123; echo unsafe"}); err == nil {
		t.Fatal("invalid window accepted")
	}
	if err := NewX11Backend().Paste(context.Background(), Window{}); err == nil {
		t.Fatal("absent target accepted")
	}
}

func TestX11PasteDoesNotInjectWhenFocusRestoreFails(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "injected")
	t.Setenv("INJECTED", marker)
	script := "#!/bin/sh\ncase \"$1\" in windowactivate) exit 1;; key) touch \"$INJECTED\";; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "xdotool"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := NewX11Backend().Paste(context.Background(), Window{ID: "42"}); err == nil {
		t.Fatal("focus failure ignored")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("injected after focus failure")
	}
}
