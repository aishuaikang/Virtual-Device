package main

import (
	"context"
	"strings"
	"testing"

	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/scenes"
)

func sceneTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	manager, err := scenes.NewManager()
	if err != nil {
		t.Fatal(err)
	}
	app := &App{sceneManager: manager}
	app.setConfigBaseline(config.DefaultConfig(), "")
	return app
}

func TestSaveSceneRequiresOverwriteConfirmation(t *testing.T) {
	app := sceneTestApp(t)
	original := config.DefaultConfig()
	if err := app.SaveScene("Demo", *original, false); err != nil {
		t.Fatal(err)
	}
	changed := cloneConfig(original)
	changed.MinPushSpeed++
	app.setCurrentConfig(changed)
	if err := app.SaveScene("Demo", *changed, false); err == nil {
		t.Fatal("saved an existing scene without confirmation")
	}
	stored, err := app.sceneManager.Load("Demo")
	if err != nil || stored.MinPushSpeed != original.MinPushSpeed || !app.dirty {
		t.Fatalf("rejected overwrite changed stored config or dirty state: %v", err)
	}
	if err := app.SaveScene("Demo", *changed, true); err != nil {
		t.Fatal(err)
	}
	stored, err = app.sceneManager.Load("Demo")
	if err != nil || stored.MinPushSpeed != changed.MinPushSpeed || app.dirty {
		t.Fatalf("confirmed overwrite did not update config and baseline: %v", err)
	}
}

func TestPrepareCloseSavesCurrentScene(t *testing.T) {
	app := sceneTestApp(t)
	cfg := config.DefaultConfig()
	if err := app.SaveScene("Demo", *cfg, false); err != nil {
		t.Fatal(err)
	}
	cfg.MinPushSpeed++
	app.setCurrentConfig(cfg)
	if err := app.prepareClose(true); err != nil {
		t.Fatal(err)
	}
	stored, err := app.sceneManager.Load("Demo")
	if err != nil || stored.MinPushSpeed != cfg.MinPushSpeed {
		t.Fatalf("exit lost the latest config: %v", err)
	}
	if app.dirty || !app.closeApproved || app.beforeClose(context.Background()) {
		t.Fatal("successful save must allow the following close")
	}
}

func TestPrepareCloseCreatesLocalizedScene(t *testing.T) {
	app := sceneTestApp(t)
	app.SetLanguage("fr")
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed++
	app.setCurrentConfig(cfg)
	if err := app.prepareClose(true); err != nil {
		t.Fatal(err)
	}
	names, err := app.sceneManager.List()
	if err != nil || len(names) != 1 || !strings.HasPrefix(names[0], nativeTranslations["fr"].saveName) {
		t.Fatalf("new exit scene was not created in the selected language: %v, %v", names, err)
	}
	if got := app.nextCloseSaveSceneName(names[0]); got != names[0]+" (1)" {
		t.Fatalf("automatic exit scene would overwrite an existing scene: %q", got)
	}
}

func TestPrepareCloseDiscardDoesNotSave(t *testing.T) {
	app := sceneTestApp(t)
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed++
	app.setCurrentConfig(cfg)
	if err := app.prepareClose(false); err != nil {
		t.Fatal(err)
	}
	names, err := app.sceneManager.List()
	if err != nil || len(names) != 0 || !app.closeApproved {
		t.Fatalf("discard saved a scene or prevented closing: %v, %v", names, err)
	}
}

func TestPrepareCloseSaveFailureKeepsDraft(t *testing.T) {
	app := sceneTestApp(t)
	cfg := config.DefaultConfig()
	cfg.MinPushSpeed++
	app.setCurrentConfig(cfg)
	app.sceneManager = nil
	if err := app.prepareClose(true); err == nil {
		t.Fatal("save failure was ignored")
	}
	if app.closeApproved || !app.dirty || app.currentCfg.MinPushSpeed != cfg.MinPushSpeed {
		t.Fatal("save failure approved closing or lost the draft")
	}
}

func TestCleanConfigClosesWithoutConfirmation(t *testing.T) {
	app := sceneTestApp(t)
	if app.beforeClose(context.Background()) {
		t.Fatal("unchanged config should close without confirmation")
	}
}
