package template

import (
	"os"
	"path/filepath"
	"testing"
)

// writeToml writes a spin.toml with the given pre/post inline steps under dir.
func writeToml(t *testing.T, dir string, pres, posts []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "_base"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "spin.toml"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if _, err := f.WriteString("name = \"t\"\n\n"); err != nil {
		t.Fatal(err)
	}
	for _, r := range pres {
		if _, err := f.WriteString("[[pre]]\nrun = " + quote(r) + "\n\n"); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range posts {
		if _, err := f.WriteString("[[post]]\nrun = " + quote(r) + "\n\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func quote(s string) string {
	return "\"" + s + "\""
}

func TestCollectHooksInlineOnly(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, []string{"echo pre"}, []string{"echo post1", "echo post2"})

	tpl, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := CollectHooks(tpl)
	want := []string{"pre:echo pre", "post:echo post1", "post:echo post2"}
	if len(got) != len(want) {
		t.Fatalf("got %d hooks, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		phase := g.Phase
		run := g.Run
		if g.IsFile {
			t.Fatalf("hook %d should not be a file", i)
		}
		if phase+":"+run != w {
			t.Errorf("hook %d = %s:%s, want %s", i, phase, run, w)
		}
	}
}

func TestCollectHooksFileDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, nil, []string{"echo post"})

	if err := os.MkdirAll(filepath.Join(dir, "_pre"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "_post"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "_pre", "b.sh"), []byte("echo b"), 0o644)
	os.WriteFile(filepath.Join(dir, "_pre", "a.sh"), []byte("echo a"), 0o644)
	os.WriteFile(filepath.Join(dir, "_post", "z.sh"), []byte("echo z"), 0o644)

	tpl, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := CollectHooks(tpl)

	// Expected order: inline pre (none), _pre files sorted (a then b),
	// inline post (echo post), _post files sorted (z).
	want := []struct {
		phase  string
		isFile bool
		tail   string
	}{
		{"pre", true, "a.sh"},
		{"pre", true, "b.sh"},
		{"post", false, ""},
		{"post", true, "z.sh"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d hooks, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Phase != w.phase || g.IsFile != w.isFile {
			t.Errorf("hook %d = phase=%s isFile=%v, want phase=%s isFile=%v",
				i, g.Phase, g.IsFile, w.phase, w.isFile)
		}
		if w.isFile && !filepathHasSuffix(g.File, w.tail) {
			t.Errorf("hook %d file = %s, want suffix %s", i, g.File, w.tail)
		}
	}
}

func TestCollectHooksSkipsEmptyAndMissingDirs(t *testing.T) {
	dir := t.TempDir()
	// Only one inline pre; no post steps, no _pre/_post dirs.
	writeToml(t, dir, []string{"real"}, nil)

	tpl, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := CollectHooks(tpl)
	if len(got) != 1 || got[0].Run != "real" {
		t.Fatalf("got %+v, want single 'real' pre hook", got)
	}
}

func filepathHasSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}
