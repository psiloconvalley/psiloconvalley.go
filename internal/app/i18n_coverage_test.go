package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"psiloconvalley/internal/i18n"
)

// TestI18nKeysCoverage walks every .tmpl file, extracts every .T.FieldName
// reference, and fails if any referenced field is missing from the
// i18n.T struct. This prevents the class of bug that caused
// the 37-day client-edit outage (Session 28): template references a field
// that doesn't exist on the struct, which crashes html/template at render
// with "can't evaluate field X in type interface {}".
func TestI18nKeysCoverage(t *testing.T) {
	defined := make(map[string]bool)
	v := reflect.TypeOf(i18n.T{})
	for i := 0; i < v.NumField(); i++ {
		defined[v.Field(i).Name] = true
	}

	templatesDir := findTemplatesDir(t)
	refRegex := regexp.MustCompile(`\.T\.([A-Z][A-Za-z0-9]+)`)

	type miss struct{ file, field string }
	var misses []miss

	err := filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range refRegex.FindAllStringSubmatch(string(content), -1) {
			if !defined[m[1]] {
				misses = append(misses, miss{path, m[1]})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
	if len(misses) == 0 {
		return
	}

	byField := make(map[string][]string)
	for _, m := range misses {
		byField[m.field] = append(byField[m.field], m.file)
	}
	keys := make([]string, 0, len(byField))
	for k := range byField {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("i18n keys referenced by templates but missing from i18n.T struct:\n\n")
	for _, k := range keys {
		b.WriteString("  .T." + k + "\n")
		for _, f := range byField[k] {
			b.WriteString("    - " + f + "\n")
		}
	}
	b.WriteString("\nFix: add each field to internal/i18n/i18n.go,\nthen populate internal/i18n/en.go and internal/i18n/es.go.")
	t.Fatal(b.String())
}

func findTemplatesDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "templates")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate templates/ directory from %s", cwd)
	return ""
}
