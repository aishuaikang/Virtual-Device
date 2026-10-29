package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestFileDialogLanguage(t *testing.T) {
	for _, test := range []struct{ language, title string }{
		{"zh", "导入场景"}, {"en", "Import scene"},
		{"tr", "Senaryo içe aktar"}, {"fr", "Importer un scénario"},
	} {
		t.Run(test.language, func(t *testing.T) {
			t.Parallel()
			app := &App{}
			app.SetLanguage(test.language)
			text := app.dialogMessages()
			if text.importTitle != test.title {
				t.Fatalf("import title = %q, want %q", text.importTitle, test.title)
			}
			value := reflect.ValueOf(text)
			for index := range value.NumField() {
				if strings.TrimSpace(value.Field(index).String()) == "" {
					t.Errorf("missing file dialog translation: %s", value.Type().Field(index).Name)
				}
			}
		})
	}
}

func TestUnsupportedNativeLanguage(t *testing.T) {
	app := &App{}
	if app.dialogMessages().importTitle != nativeTranslations["en"].importTitle {
		t.Fatal("file dialogs must have an English fallback before the frontend is ready")
	}
	app.SetLanguage("fr")
	app.SetLanguage("unknown")
	if app.dialogMessages().importTitle != nativeTranslations["fr"].importTitle {
		t.Fatal("an unsupported language changed the selected file dialog language")
	}
}
