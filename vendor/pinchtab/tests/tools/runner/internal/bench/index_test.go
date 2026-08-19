package bench

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseIndexAllActive(t *testing.T) {
	content := `# Benchmark Run

## Group Files

- group-00.md — Setup
- group-01.md — Reading
- group-02.md — Search
`
	idx, err := parseIndexContent("/fake/dir", content)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 2}
	if !reflect.DeepEqual(idx.Active, want) {
		t.Errorf("Active = %v; want %v", idx.Active, want)
	}
	if len(idx.Disabled) != 0 {
		t.Errorf("Disabled = %v; want empty", idx.Disabled)
	}
}

func TestParseIndexSingleLineComment(t *testing.T) {
	content := `## Group Files

- group-00.md — Setup
<!-- - group-01.md — Reading (disabled) -->
- group-02.md — Search
`
	idx, err := parseIndexContent("/fake/dir", content)
	if err != nil {
		t.Fatal(err)
	}
	wantActive := []int{0, 2}
	wantDisabled := []int{1}
	if !reflect.DeepEqual(idx.Active, wantActive) {
		t.Errorf("Active = %v; want %v", idx.Active, wantActive)
	}
	if !reflect.DeepEqual(idx.Disabled, wantDisabled) {
		t.Errorf("Disabled = %v; want %v", idx.Disabled, wantDisabled)
	}
}

func TestParseIndexMultiLineComment(t *testing.T) {
	content := `## Group Files

- group-00.md — Setup
<!--
- group-01.md — Reading (disabled)
- group-02.md — Search (disabled)
-->
- group-03.md — Form
`
	idx, err := parseIndexContent("/fake/dir", content)
	if err != nil {
		t.Fatal(err)
	}
	wantActive := []int{0, 3}
	wantDisabled := []int{1, 2}
	if !reflect.DeepEqual(idx.Active, wantActive) {
		t.Errorf("Active = %v; want %v", idx.Active, wantActive)
	}
	if !reflect.DeepEqual(idx.Disabled, wantDisabled) {
		t.Errorf("Disabled = %v; want %v", idx.Disabled, wantDisabled)
	}
}

func TestParseIndexEmptyIndex(t *testing.T) {
	content := `# Empty Index

No group section here.
`
	idx, err := parseIndexContent("/fake/dir", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Active) != 0 || len(idx.Disabled) != 0 {
		t.Errorf("Expected empty sets; got Active=%v Disabled=%v", idx.Active, idx.Disabled)
	}
}

func TestParseIndexBacktickFilenames(t *testing.T) {
	content := `## Group Files

- ` + "`group-00.md`" + ` — Setup
<!-- - ` + "`group-01.md`" + ` — disabled -->
`
	idx, err := parseIndexContent("/fake/dir", content)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(idx.Active, []int{0}) {
		t.Errorf("Active = %v; want [0]", idx.Active)
	}
	if !reflect.DeepEqual(idx.Disabled, []int{1}) {
		t.Errorf("Disabled = %v; want [1]", idx.Disabled)
	}
}

func TestScanGroupFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"group-00.md", "group-05.md", "group-10.md", "setup.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	groups, err := ScanGroupFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 5, 10}
	if !reflect.DeepEqual(groups, want) {
		t.Errorf("ScanGroupFiles = %v; want %v", groups, want)
	}
}

func TestSelectGroupsExplicit(t *testing.T) {
	idx := IndexFile{Dir: "/d", Active: []int{0, 1, 2}, Disabled: []int{3}}
	args := Args{Groups: []int{1, 3}}
	var stderr bytes.Buffer
	sel, err := SelectGroups(args, idx, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 3}
	if !reflect.DeepEqual(sel.Selected, want) {
		t.Errorf("Selected = %v; want %v", sel.Selected, want)
	}
	if len(sel.Warnings) != 1 {
		t.Errorf("Expected 1 warning; got %d", len(sel.Warnings))
	}
}

func TestSelectGroupsProfile(t *testing.T) {
	idx := IndexFile{Dir: "/d", Active: []int{0, 1, 2, 3, 4, 5}}
	args := Args{Profile: "common10"}
	var stderr bytes.Buffer
	sel, err := SelectGroups(args, idx, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 1, 2, 3}
	if !reflect.DeepEqual(sel.Selected, want) {
		t.Errorf("Selected = %v; want %v", sel.Selected, want)
	}
}

func TestSelectGroupsActiveOnly(t *testing.T) {
	idx := IndexFile{Dir: "/d", Active: []int{0, 2, 4}}
	args := Args{}
	var stderr bytes.Buffer
	sel, err := SelectGroups(args, idx, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sel.Selected, idx.Active) {
		t.Errorf("Selected = %v; want %v", sel.Selected, idx.Active)
	}
}
