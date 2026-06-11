package scenes

import (
	"os"
	"path/filepath"
	"testing"

	"virtual-device-ui/internal/config"
)

func TestExportAddsJSONExtension(t *testing.T) {
	manager := &Manager{dir: t.TempDir()}
	cfg := config.DefaultConfig()
	if err := manager.Save("demo", cfg); err != nil {
		t.Fatalf("save scene: %v", err)
	}

	exportPath, err := manager.Export("demo", filepath.Join(t.TempDir(), "scene-backup"))
	if err != nil {
		t.Fatalf("export scene: %v", err)
	}

	if filepath.Ext(exportPath) != ".json" {
		t.Fatalf("expected .json extension, got %q", exportPath)
	}
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("exported file missing: %v", err)
	}
}

func TestImportUsesUniqueSceneName(t *testing.T) {
	manager := &Manager{dir: t.TempDir()}
	cfg := config.DefaultConfig()
	if err := manager.Save("backup", cfg); err != nil {
		t.Fatalf("save original scene: %v", err)
	}

	importFile := filepath.Join(t.TempDir(), "backup.json")
	if _, err := manager.Export("backup", importFile); err != nil {
		t.Fatalf("prepare import file: %v", err)
	}

	importedName, err := manager.Import(importFile)
	if err != nil {
		t.Fatalf("import scene: %v", err)
	}

	if importedName != "backup (1)" {
		t.Fatalf("unexpected imported scene name: %q", importedName)
	}
	if _, err := manager.Load(importedName); err != nil {
		t.Fatalf("imported scene not loadable: %v", err)
	}
}

func TestSaveRejectsDuplicateScene(t *testing.T) {
	manager := &Manager{dir: t.TempDir()}
	cfg := config.DefaultConfig()
	if err := manager.Save("demo", cfg); err != nil {
		t.Fatalf("save original scene: %v", err)
	}

	if err := manager.Save("demo", cfg); err == nil {
		t.Fatal("expected duplicate scene save to fail")
	}
}

func TestRenameRejectsExistingScene(t *testing.T) {
	manager := &Manager{dir: t.TempDir()}
	cfg := config.DefaultConfig()
	if err := manager.Save("alpha", cfg); err != nil {
		t.Fatalf("save alpha: %v", err)
	}
	if err := manager.Save("beta", cfg); err != nil {
		t.Fatalf("save beta: %v", err)
	}

	if err := manager.Rename("alpha", "beta"); err == nil {
		t.Fatal("expected rename to existing scene to fail")
	}
}

func TestListReturnsSortedScenes(t *testing.T) {
	manager := &Manager{dir: t.TempDir()}
	cfg := config.DefaultConfig()
	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if err := manager.Save(name, cfg); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	names, err := manager.List()
	if err != nil {
		t.Fatalf("list scenes: %v", err)
	}

	expected := []string{"alpha", "bravo", "charlie"}
	for i, name := range expected {
		if names[i] != name {
			t.Fatalf("expected sorted scenes %v, got %v", expected, names)
		}
	}
}
