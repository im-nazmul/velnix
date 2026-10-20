//go:build linux

package projectmeta

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLiteralDetectionAndRequirements(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json":   `{"dependencies":{"next":"latest","react":"19"},"engines":{"node":">=20.0.0 <26"},"scripts":{"postinstall":"touch hook-ran"}}`,
		"composer.json":  `{"require":{"laravel/framework":"*"},"scripts":{"post-autoload-dump":"touch hook-ran"}}`,
		"pyproject.toml": "[project]\nrequires-python = '>=3.12,<3.14'\ndependencies = ['fastapi>=0.1','Django[extra]>=5','',';']\n[build-system]\nrequires=['untrusted']\nbuild-backend='untrusted'\n",
		".nvmrc":         "v25.9.0\n", ".node-version": "25.9.0", ".python-version": "3.12.3", ".tool-versions": "nodejs 25.9.0\npython 3.12.3\nbun 1.4.3\nruby 3.2.0\n",
	}
	for name, b := range files {
		if e := os.WriteFile(filepath.Join(root, name), []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	r, e := Inspect(root)
	if e != nil {
		t.Fatal(e)
	}
	want := []string{"Django", "FastAPI", "Laravel", "Next.js", "React"}
	if !reflect.DeepEqual(r.Frameworks, want) || r.Digest == "" {
		t.Fatalf("%+v", r)
	}
	for kind, v := range map[string]string{"node": "25.9.0", "bun": "1.4.3", "python": "3.12.3"} {
		if e = r.Check(kind, v); e != nil {
			t.Fatal(e)
		}
	}
	if e = r.Check("node", "24.0.0"); e == nil {
		t.Fatal("conflict ignored")
	}
	if _, e = os.Stat(filepath.Join(root, "hook-ran")); !os.IsNotExist(e) {
		t.Fatal("hook executed")
	}
	os.WriteFile(filepath.Join(root, "package.json"), []byte(files["package.json"]+"\n"), 0600)
	after, e := Inspect(root)
	if e != nil || after.Digest == r.Digest {
		t.Fatal("raw metadata edit did not change fingerprint")
	}
}
func TestUnsafeMetadataRefused(t *testing.T) {
	for _, kind := range []string{"symlink", "oversize", "fifo", "alias", "malformed-json", "malformed-toml"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "package.json")
			switch kind {
			case "symlink":
				os.Symlink("/etc/passwd", path)
			case "oversize":
				os.WriteFile(path, []byte(strings.Repeat(" ", MaxFile+1)), 0600)
			case "fifo":
				if e := makeFIFO(path); e != nil {
					t.Fatal(e)
				}
			case "alias":
				os.WriteFile(filepath.Join(root, ".nvmrc"), []byte("$(touch hook-ran)"), 0600)
			case "malformed-json":
				os.WriteFile(path, []byte("{"), 0600)
			case "malformed-toml":
				os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project"), 0600)
			}
			if _, e := Inspect(root); e == nil {
				t.Fatal("unsafe metadata accepted")
			}
		})
	}
}
func TestSupportedConstraints(t *testing.T) {
	for _, tt := range []struct {
		v, expr string
		ok      bool
	}{{"3.12.3", ">=3.10,<3.13", true}, {"3.13.0", ">=3.10,<3.13", false}, {"25.9.0", "^25.0.0", true}, {"26.0.0", "^25.0.0", false}, {"0.2.9", "^0.2.1", true}, {"0.3.0", "^0.2.1", false}, {"3.12.3", "~=3.12", true}, {"4.0.0", "~=3.12", false}, {"3.13.0", "~=3.12.1", false}, {"3.12.3", "!=3.12.*", false}, {"3.13.0", "!=3.12.*", true}, {"20.12.1", ">=18 <21", true}, {"1.4.3", "~1.4.0", true}} {
		got, e := Satisfies(tt.v, tt.expr)
		if e != nil || got != tt.ok {
			t.Fatalf("%s %s: %v %v", tt.v, tt.expr, got, e)
		}
	}
	for _, expr := range []string{"lts/*", "latest", ">=3.10 || <3.8", "$(touch x)", ""} {
		if _, e := Satisfies("3.12.3", expr); e == nil {
			t.Fatalf("unsupported expression accepted %q", expr)
		}
	}
}
