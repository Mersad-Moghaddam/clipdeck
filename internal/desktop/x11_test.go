package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.WriteFile(filepath.Join(dir, "xprop"), []byte("#!/bin/sh\nprintf 'WM_CLASS(STRING) = \"editor\", \"Editor\"\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
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

func TestX11PasteDoesNotInjectAfterLateFocusChange(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "injected")
	t.Setenv("INJECTED", marker)
	script := "#!/bin/sh\ncase \"$1\" in windowactivate) exit 0;; getactivewindow) printf '99\\n';; key) touch \"$INJECTED\";; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "xprop"), []byte("#!/bin/sh\nprintf 'WM_CLASS(STRING) = \"editor\", \"Editor\"\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "xdotool"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := NewX11Backend().Paste(context.Background(), Window{ID: "42"}); err == nil {
		t.Fatal("late focus change ignored")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("injected into another window")
	}
}

func TestX11PasteRejectsPopupTarget(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "injected")
	t.Setenv("INJECTED", marker)
	for name, script := range map[string]string{
		"xdotool": "#!/bin/sh\ncase \"$1\" in getactivewindow) printf '42\\n';; key) touch \"$INJECTED\";; esac\n",
		"xprop":   "#!/bin/sh\nprintf 'WM_CLASS(STRING) = \"clipdeck-ui\", \"clipdeck-ui\"\\n'\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := NewX11Backend().Paste(context.Background(), Window{ID: "42"}); err == nil {
		t.Fatal("popup target accepted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("injected into popup")
	}
}

func TestX11PasteInjectionIsBoundToValidatedWindow(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "keys")
	t.Setenv("KEY_LOG", log)
	for name, script := range map[string]string{
		"xdotool": "#!/bin/sh\ncase \"$1\" in getactivewindow) printf '42\\n';; key) printf '%s\\n' \"$*\" >\"$KEY_LOG\";; esac\n",
		"xprop":   "#!/bin/sh\nprintf 'WM_CLASS(STRING) = \"editor\", \"Editor\"\\n'\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := NewX11Backend().Paste(context.Background(), Window{ID: "42"}); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--window 42") {
		t.Fatalf("focus can redirect injection: %s", args)
	}
}
