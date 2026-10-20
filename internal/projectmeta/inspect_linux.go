//go:build linux

package projectmeta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"velnix/internal/domain"
)

type Requirement struct {
	Kind       string `json:"kind"`
	Expression string `json:"expression"`
	Source     string `json:"source"`
}
type Report struct {
	Digest       string        `json:"digest"`
	Frameworks   []string      `json:"frameworks"`
	Requirements []Requirement `json:"requirements"`
	Evidence     []string      `json:"evidence"`
}

func Inspect(root string) (Report, error) {
	r := Report{Frameworks: []string{}, Requirements: []Requirement{}, Evidence: []string{}}
	detected := map[string]bool{}
	fingerprints := []string{}
	read := func(name string) ([]byte, error) {
		b, e := Read(root, name)
		if os.IsNotExist(e) {
			return nil, nil
		}
		if e != nil {
			return nil, domain.E(domain.Invalid, "cannot inspect %s safely: %v", name, e)
		}
		if b != nil {
			r.Evidence = append(r.Evidence, name)
			h := sha256.Sum256(b)
			fingerprints = append(fingerprints, name+":"+hex.EncodeToString(h[:]))
		}
		return b, nil
	}
	add := func(kind, expr, source string) {
		r.Requirements = append(r.Requirements, Requirement{kind, strings.TrimSpace(expr), source})
	}
	for _, item := range []struct{ name, kind string }{{".nvmrc", "node"}, {".node-version", "node"}, {".python-version", "python"}, {".bun-version", "bun"}} {
		b, e := read(item.name)
		if e != nil {
			return r, e
		}
		if b != nil {
			v := strings.TrimSpace(string(b))
			if item.kind == "node" {
				v = strings.TrimPrefix(v, "v")
			}
			if !domain.ValidRuntimeVersion(v) {
				return r, domain.E(domain.Invalid, "%s requires one literal exact version; aliases, expressions and multiple versions are unsupported", item.name)
			}
			add(item.kind, v, item.name)
		}
	}
	if b, e := read(".tool-versions"); e != nil {
		return r, e
	} else if b != nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(strings.SplitN(line, "#", 2)[0])
			if len(fields) == 0 {
				continue
			}
			kind := fields[0]
			if kind == "nodejs" {
				kind = "node"
			}
			if !domain.MultiRuntime(kind) {
				continue
			}
			if len(fields) != 2 || seen[kind] || !domain.ValidRuntimeVersion(strings.TrimPrefix(fields[1], "v")) {
				return r, domain.E(domain.Invalid, ".tool-versions requires one exact version per supported runtime")
			}
			seen[kind] = true
			add(kind, strings.TrimPrefix(fields[1], "v"), ".tool-versions")
		}
	}
	if b, e := read("package.json"); e != nil {
		return r, e
	} else if b != nil {
		var p struct {
			Dependencies    map[string]json.RawMessage
			DevDependencies map[string]json.RawMessage
			Engines         map[string]string
		}
		if e = json.Unmarshal(b, &p); e != nil {
			return r, domain.E(domain.Invalid, "invalid package.json: %v", e)
		}
		for _, kind := range []string{"node", "bun"} {
			if expr, ok := p.Engines[kind]; ok {
				add(kind, expr, "package.json engines."+kind)
			}
		}
		for _, deps := range []map[string]json.RawMessage{p.Dependencies, p.DevDependencies} {
			for key := range deps {
				switch key {
				case "next":
					detected["Next.js"] = true
				case "nuxt":
					detected["Nuxt"] = true
				case "vue":
					detected["Vue"] = true
				case "react":
					detected["React"] = true
				case "vite":
					detected["Vite"] = true
				case "express":
					detected["Express"] = true
				case "fastify":
					detected["Fastify"] = true
				case "@angular/core":
					detected["Angular"] = true
				}
			}
		}
	}
	if b, e := read("composer.json"); e != nil {
		return r, e
	} else if b != nil {
		var p struct{ Require map[string]json.RawMessage }
		if e = json.Unmarshal(b, &p); e != nil {
			return r, domain.E(domain.Invalid, "invalid composer.json: %v", e)
		}
		for key := range p.Require {
			if key == "laravel/framework" {
				detected["Laravel"] = true
			}
			if key == "symfony/framework-bundle" {
				detected["Symfony"] = true
			}
		}
	}
	if b, e := read("pyproject.toml"); e != nil {
		return r, e
	} else if b != nil {
		var p struct {
			Project struct {
				RequiresPython string `toml:"requires-python"`
				Dependencies   []string
			}
			Tool struct {
				Poetry struct{ Dependencies map[string]any }
			}
		}
		if e = toml.Unmarshal(b, &p); e != nil {
			return r, domain.E(domain.Invalid, "invalid pyproject.toml: %v", e)
		}
		if p.Project.RequiresPython != "" {
			add("python", p.Project.RequiresPython, "pyproject.toml project.requires-python")
		}
		if value, ok := p.Tool.Poetry.Dependencies["python"]; ok {
			v, ok := value.(string)
			if !ok {
				return r, domain.E(domain.Invalid, "Poetry Python requirement must be literal text")
			}
			add("python", v, "pyproject.toml tool.poetry.dependencies.python")
		}
		deps := append([]string(nil), p.Project.Dependencies...)
		for key := range p.Tool.Poetry.Dependencies {
			deps = append(deps, key)
		}
		for _, dep := range deps {
			key := strings.ToLower(strings.TrimSpace(dep))
			fields := strings.FieldsFunc(key, func(c rune) bool { return strings.ContainsRune("[<>=!~; ", c) })
			if len(fields) == 0 {
				continue
			}
			key = fields[0]
			switch key {
			case "django":
				detected["Django"] = true
			case "flask":
				detected["Flask"] = true
			case "fastapi":
				detected["FastAPI"] = true
			}
		}
	}
	for key := range detected {
		r.Frameworks = append(r.Frameworks, key)
	}
	sort.Strings(r.Frameworks)
	h := sha256.Sum256([]byte(strings.Join(fingerprints, "\n")))
	r.Digest = hex.EncodeToString(h[:])
	return r, nil
}

// Check requires an exact selected version to satisfy every recognized literal
// file and supported manifest constraint. It never chooses a fallback version.
func (r Report) Check(kind, version string) error {
	for _, req := range r.Requirements {
		if req.Kind != kind {
			continue
		}
		ok, e := Satisfies(version, req.Expression)
		if e != nil {
			return domain.E(domain.Invalid, "unsupported %s requirement in %s: %v", kind, req.Source, e)
		}
		if !ok {
			return domain.E(domain.Conflict, "selected %s %s conflicts with %s requirement %q", kind, version, req.Source, req.Expression)
		}
	}
	return nil
}
func numbers(v string) ([3]int, int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return out, 0, fmt.Errorf("expected numeric version")
	}
	for i, p := range parts {
		if p == "" {
			return out, 0, fmt.Errorf("empty version component")
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return out, 0, fmt.Errorf("only numeric release constraints are supported")
			}
		}
		n, e := strconv.Atoi(p)
		if e != nil || n > 1000000 {
			return out, 0, fmt.Errorf("invalid version component")
		}
		out[i] = n
	}
	return out, len(parts), nil
}
func compare(a, b [3]int) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
func Satisfies(version, expr string) (bool, error) {
	if expr == "*" || expr == "x" {
		return domain.ValidRuntimeVersion(version), nil
	}
	if version == expr {
		return true, nil
	}
	v, _, e := numbers(version)
	if e != nil {
		return false, e
	}
	terms := strings.Fields(strings.ReplaceAll(strings.TrimSpace(expr), ",", " "))
	if len(terms) == 0 {
		return false, fmt.Errorf("empty constraint")
	}
	for _, term := range terms {
		op := "="
		for _, candidate := range []string{"~=", ">=", "<=", "==", "!=", ">", "<", "^", "~", "="} {
			if strings.HasPrefix(term, candidate) {
				op = candidate
				term = strings.TrimPrefix(term, candidate)
				break
			}
		}
		wildcard := strings.HasSuffix(term, ".*") || strings.HasSuffix(term, ".x")
		if wildcard {
			term = term[:len(term)-2]
		}
		value, parts, e := numbers(term)
		if e != nil {
			return false, e
		}
		cmp := compare(v, value)
		ok := false
		switch op {
		case ">=":
			ok = cmp >= 0
		case "<=":
			ok = cmp <= 0
		case ">":
			ok = cmp > 0
		case "<":
			ok = cmp < 0
		case "=", "==", "!=":
			ok = cmp == 0
			if parts < 3 || wildcard {
				ok = true
				for i := 0; i < parts; i++ {
					ok = ok && v[i] == value[i]
				}
			}
			if op == "!=" {
				ok = !ok
			}
		case "~", "~=", "^":
			if op == "~=" && parts < 2 {
				return false, fmt.Errorf("compatible-release requires at least two version components")
			}
			upper := value
			index := 1
			if parts == 1 {
				index = 0
			}
			if op == "~=" && parts == 2 {
				index = 0
			}
			if op == "^" {
				index = 0
				if value[0] == 0 && parts > 1 {
					index = 1
					if value[1] == 0 && parts > 2 {
						index = 2
					}
				}
			}
			upper[index]++
			for i := index + 1; i < 3; i++ {
				upper[i] = 0
			}
			ok = cmp >= 0 && compare(v, upper) < 0
		}
		if wildcard && op != "=" && op != "==" && op != "!=" {
			return false, fmt.Errorf("wildcard comparator unsupported")
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}
