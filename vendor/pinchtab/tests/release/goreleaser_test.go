package release

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

type goReleaserConfig struct {
	Builds []struct {
		Main   string   `yaml:"main"`
		GOOS   []string `yaml:"goos"`
		GOARCH []string `yaml:"goarch"`
	} `yaml:"builds"`
	Archives []struct {
		Format       string `yaml:"format"`
		NameTemplate string `yaml:"name_template"`
	} `yaml:"archives"`
}

func TestGoReleaserBuildMatrix(t *testing.T) {
	cfg := loadGoReleaserConfig(t)

	build, ok := findPinchTabBuild(cfg)
	if !ok {
		t.Fatal("missing build config for ./cmd/pinchtab in .goreleaser.yml")
	}

	wantOS := []string{"darwin", "linux", "windows"}
	wantArch := []string{"amd64", "arm64"}

	gotOS := slices.Clone(build.GOOS)
	gotArch := slices.Clone(build.GOARCH)
	slices.Sort(gotOS)
	slices.Sort(gotArch)

	if !slices.Equal(gotOS, wantOS) {
		t.Fatalf("unexpected goos matrix: got %v want %v", gotOS, wantOS)
	}
	if !slices.Equal(gotArch, wantArch) {
		t.Fatalf("unexpected goarch matrix: got %v want %v", gotArch, wantArch)
	}

	total := len(build.GOOS) * len(build.GOARCH)
	if total != 6 {
		t.Fatalf("unexpected binary count: got %d want 6", total)
	}
}

func TestGoReleaserBinaryNaming(t *testing.T) {
	cfg := loadGoReleaserConfig(t)

	build, ok := findPinchTabBuild(cfg)
	if !ok {
		t.Fatal("missing build config for ./cmd/pinchtab in .goreleaser.yml")
	}

	archive, ok := findBinaryArchive(cfg)
	if !ok {
		t.Fatal("missing binary archive config in .goreleaser.yml")
	}

	if archive.NameTemplate != "pinchtab-{{ .Os }}-{{ .Arch }}" {
		t.Fatalf("unexpected binary name template: got %q", archive.NameTemplate)
	}

	var got []string
	for _, goos := range build.GOOS {
		for _, goarch := range build.GOARCH {
			name := "pinchtab-" + goos + "-" + goarch
			if goos == "windows" {
				name += ".exe"
			}
			got = append(got, name)
		}
	}
	slices.Sort(got)

	want := []string{
		"pinchtab-darwin-amd64",
		"pinchtab-darwin-arm64",
		"pinchtab-linux-amd64",
		"pinchtab-linux-arm64",
		"pinchtab-windows-amd64.exe",
		"pinchtab-windows-arm64.exe",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("unexpected artifact names: got %v want %v", got, want)
	}
}

func loadGoReleaserConfig(t *testing.T) goReleaserConfig {
	t.Helper()

	configPath := filepath.Join("..", "..", ".goreleaser.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", configPath, err)
	}

	var cfg goReleaserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse %s: %v", configPath, err)
	}

	return cfg
}

func findPinchTabBuild(cfg goReleaserConfig) (struct {
	Main   string   "yaml:\"main\""
	GOOS   []string "yaml:\"goos\""
	GOARCH []string "yaml:\"goarch\""
}, bool) {
	for _, build := range cfg.Builds {
		if build.Main == "./cmd/pinchtab" {
			return build, true
		}
	}
	return struct {
		Main   string   "yaml:\"main\""
		GOOS   []string "yaml:\"goos\""
		GOARCH []string "yaml:\"goarch\""
	}{}, false
}

func findBinaryArchive(cfg goReleaserConfig) (struct {
	Format       string "yaml:\"format\""
	NameTemplate string "yaml:\"name_template\""
}, bool) {
	for _, archive := range cfg.Archives {
		if archive.Format == "binary" {
			return archive, true
		}
	}
	return struct {
		Format       string "yaml:\"format\""
		NameTemplate string "yaml:\"name_template\""
	}{}, false
}
