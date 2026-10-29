package main

type nativeMessages struct {
	saveName    string
	importTitle string
	exportTitle string
	jsonFiles   string
}

var nativeTranslations = map[string]nativeMessages{
	"en": {saveName: "Saved before exit", importTitle: "Import scene", exportTitle: "Export scene", jsonFiles: "JSON files (*.json)"},
	"zh": {saveName: "退出前保存", importTitle: "导入场景", exportTitle: "导出场景", jsonFiles: "JSON 文件 (*.json)"},
	"tr": {saveName: "Çıkmadan önce kaydedildi", importTitle: "Senaryo içe aktar", exportTitle: "Senaryo dışa aktar", jsonFiles: "JSON dosyaları (*.json)"},
	"fr": {saveName: "Enregistré avant de quitter", importTitle: "Importer un scénario", exportTitle: "Exporter un scénario", jsonFiles: "Fichiers JSON (*.json)"},
}

// SetLanguage synchronizes file dialogs and generated scene names with the interface language.
func (a *App) SetLanguage(language string) {
	if _, ok := nativeTranslations[language]; !ok {
		return
	}
	a.stateMu.Lock()
	a.lang = language
	a.stateMu.Unlock()
}

func (a *App) dialogMessages() nativeMessages {
	a.stateMu.RLock()
	language := a.lang
	a.stateMu.RUnlock()
	if messages, ok := nativeTranslations[language]; ok {
		return messages
	}
	return nativeTranslations["en"]
}
