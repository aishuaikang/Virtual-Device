package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"virtual-device-ui/internal/config"
	"virtual-device-ui/internal/engine"
	"virtual-device-ui/internal/scenes"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	engine        *engine.Engine
	sceneManager  *scenes.Manager
	logWriter     *engine.LogWriter
	httpClient    *http.Client
	searchMu      sync.Mutex
	searchCache   map[string][]LocationSearchResult
	nextSearchAt  time.Time
	stateMu       sync.RWMutex
	currentCfg    *config.Config
	baselineCfg   *config.Config
	selectedScene string
	dirty         bool
	lang          string
}

type LocationSearchResult struct {
	DisplayName string  `json:"displayName"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
}

type nominatimSearchResult struct {
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}

const (
	closeSaveAndExitButton    = "Save & Exit / 保存并退出"
	closeDiscardAndExitButton = "Discard / 直接退出"
	closeCancelButton         = "Cancel / 取消"
	overwriteSceneButton      = "Replace / 覆盖"
	dialogOKButton            = "OK / 确定"
)

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		copy := *cfg
		return &copy
	}

	var cloned config.Config
	if err := json.Unmarshal(data, &cloned); err != nil {
		copy := *cfg
		return &copy
	}
	return &cloned
}

func configsEqual(left, right *config.Config) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	}

	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func normalizeConfigCopy(cfg *config.Config) *config.Config {
	cloned := cloneConfig(cfg)
	config.NormalizeConfig(cloned)
	return cloned
}

func validateConfigCopy(cfg *config.Config) (*config.Config, error) {
	cloned := normalizeConfigCopy(cfg)
	if err := config.ValidateConfig(cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func geocoderSearchURL() string {
	if endpoint := strings.TrimSpace(os.Getenv("VIRTUAL_DEVICE_UI_GEOCODER_SEARCH_URL")); endpoint != "" {
		return endpoint
	}
	return "https://nominatim.openstreetmap.org/search"
}

// NewApp creates a new App application struct
func NewApp() *App {
	lw := engine.NewLogWriter(512)
	log.SetOutput(lw)
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	return &App{
		engine:      engine.New(),
		logWriter:   lw,
		httpClient:  &http.Client{Timeout: 8 * time.Second},
		searchCache: make(map[string][]LocationSearchResult),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	var err error
	a.sceneManager, err = scenes.NewManager()
	if err != nil {
		log.Printf("[App] 初始化场景管理器失败: %v", err)
	}

	// 将日志转发到前端 event
	go func() {
		for line := range a.logWriter.Ch {
			wailsruntime.EventsEmit(ctx, "log", line)
		}
	}()
}

// ---- 引擎控制 ----

// StartEngine 启动虚拟设备引擎
func (a *App) StartEngine(cfg config.Config) error {
	validatedCfg, err := validateConfigCopy(&cfg)
	if err != nil {
		return err
	}
	a.setCurrentConfig(validatedCfg)
	return a.engine.Start(validatedCfg)
}

// StopEngine 停止虚拟设备引擎
func (a *App) StopEngine() {
	a.engine.Stop()
}

// GetStatus 获取引擎运行状态
func (a *App) GetStatus() engine.Status {
	return a.engine.GetStatus()
}

// ---- 配置 ----

// GetDefaultConfig 返回默认配置
func (a *App) GetDefaultConfig() config.Config {
	cfg := config.DefaultConfig()
	a.setConfigBaseline(cfg, "")
	return *cfg
}

// SyncCurrentConfig 同步当前编辑中的配置到后端，供关闭拦截与保存逻辑使用。
func (a *App) SyncCurrentConfig(cfg config.Config) {
	normalizedCfg := normalizeConfigCopy(&cfg)
	a.setCurrentConfig(normalizedCfg)
}

func (a *App) setCurrentConfig(cfg *config.Config) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.currentCfg = normalizeConfigCopy(cfg)
	a.dirty = !configsEqual(a.currentCfg, a.baselineCfg)
}

func (a *App) setConfigBaseline(cfg *config.Config, selectedScene string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.currentCfg = normalizeConfigCopy(cfg)
	a.baselineCfg = normalizeConfigCopy(cfg)
	a.selectedScene = selectedScene
	a.dirty = false
}

func (a *App) clearSelectedScene(name string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.selectedScene == name {
		a.selectedScene = ""
	}
}

func (a *App) renameSelectedScene(oldName, newName string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.selectedScene == oldName {
		a.selectedScene = newName
	}
}

func (a *App) closeStateSnapshot() (*config.Config, *config.Config, string, bool) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return cloneConfig(a.currentCfg), cloneConfig(a.baselineCfg), a.selectedScene, a.dirty
}

func (a *App) nextCloseSaveSceneName(base string) string {
	if a.sceneManager == nil {
		return base
	}

	names, err := a.sceneManager.List()
	if err != nil {
		return base
	}

	existing := make(map[string]struct{}, len(names))
	for _, name := range names {
		existing[name] = struct{}{}
	}

	if _, exists := existing[base]; !exists {
		return base
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)", base, i)
		if _, exists := existing[candidate]; !exists {
			return candidate
		}
	}
}

func (a *App) savePreset(name string, cfg *config.Config) error {
	if a.sceneManager == nil {
		return fmt.Errorf("场景管理器未初始化")
	}
	validatedCfg, err := validateConfigCopy(cfg)
	if err != nil {
		return err
	}
	if err := a.sceneManager.Replace(name, validatedCfg); err != nil {
		return err
	}
	a.setConfigBaseline(validatedCfg, strings.TrimSpace(name))
	return nil
}

// beforeClose 在应用关闭前触发，返回 true 会阻止关闭。
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	currentCfg, baselineCfg, selectedScene, dirty := a.closeStateSnapshot()
	if !dirty || configsEqual(currentCfg, baselineCfg) {
		return false
	}

	targetScene := strings.TrimSpace(selectedScene)
	autoCreateScene := false
	if targetScene == "" {
		autoCreateScene = true
		targetScene = a.nextCloseSaveSceneName("退出前保存 " + time.Now().Format("2006-01-02 15-04-05"))
	}

	message := fmt.Sprintf("Configuration has changed. Save it to preset \"%s\" before exit?\n当前配置已修改，是否在退出前保存到预设“%s”？", targetScene, targetScene)
	if autoCreateScene {
		message = fmt.Sprintf("Configuration has changed. Save it as a new preset before exit?\nIf saved, preset \"%s\" will be created.\n当前配置已修改，是否在退出前保存为预设？\n若保存，将创建预设“%s”。", targetScene, targetScene)
	}

	choice, err := wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Save Preset? / 保存预设？",
		Message:       message,
		Buttons:       []string{closeSaveAndExitButton, closeDiscardAndExitButton, closeCancelButton},
		DefaultButton: closeSaveAndExitButton,
		CancelButton:  closeCancelButton,
	})
	if err != nil {
		return true
	}

	switch choice {
	case closeSaveAndExitButton:
		if currentCfg == nil {
			return false
		}
		if err := a.savePreset(targetScene, currentCfg); err != nil {
			_, _ = wailsruntime.MessageDialog(ctx, wailsruntime.MessageDialogOptions{
				Type:          wailsruntime.ErrorDialog,
				Title:         "Save Failed / 保存失败",
				Message:       fmt.Sprintf("Failed to save preset before exit: %v\n退出前保存预设失败：%v", err, err),
				Buttons:       []string{dialogOKButton},
				DefaultButton: dialogOKButton,
				CancelButton:  dialogOKButton,
			})
			return true
		}
		return false
	case closeDiscardAndExitButton:
		return false
	default:
		return true
	}
}

func resolveDialogDefaults(currentPath string) (string, string) {
	if currentPath == "" {
		return "", ""
	}

	info, err := os.Stat(currentPath)
	if err == nil {
		if info.IsDir() {
			return currentPath, ""
		}
		return filepath.Dir(currentPath), filepath.Base(currentPath)
	}

	defaultDir := filepath.Dir(currentPath)
	if defaultDir == "." || defaultDir == currentPath {
		return "", ""
	}

	if info, err := os.Stat(defaultDir); err == nil && info.IsDir() {
		return defaultDir, filepath.Base(currentPath)
	}

	return "", ""
}

// SelectFile 打开系统文件选择器
func (a *App) SelectFile(currentPath, title string) (string, error) {
	defaultDir, defaultName := resolveDialogDefaults(currentPath)
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDir,
		DefaultFilename:  defaultName,
	})
}

// SelectDirectory 打开系统目录选择器
func (a *App) SelectDirectory(currentPath, title string) (string, error) {
	defaultDir, _ := resolveDialogDefaults(currentPath)
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDir,
	})
}

// SearchPlaces 根据地区名称搜索位置
func (a *App) SearchPlaces(query string) ([]LocationSearchResult, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, nil
	}

	requestCtx := a.ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	cacheKey := strings.ToLower(trimmedQuery)

	a.searchMu.Lock()
	if cached, ok := a.searchCache[cacheKey]; ok {
		result := append([]LocationSearchResult(nil), cached...)
		a.searchMu.Unlock()
		return result, nil
	}

	now := time.Now()
	wait := time.Duration(0)
	if now.Before(a.nextSearchAt) {
		wait = time.Until(a.nextSearchAt)
		a.nextSearchAt = a.nextSearchAt.Add(time.Second)
	} else {
		a.nextSearchAt = now.Add(time.Second)
	}
	a.searchMu.Unlock()

	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-requestCtx.Done():
			return nil, context.Canceled
		}
	}

	values := url.Values{}
	values.Set("q", trimmedQuery)
	values.Set("format", "jsonv2")
	values.Set("limit", "5")
	values.Set("addressdetails", "1")

	requestURL := geocoderSearchURL() + "?" + values.Encode()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "virtual-device-ui/1.0 (desktop geosearch)")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoder returned status %d", resp.StatusCode)
	}

	var payload []nominatimSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	results := make([]LocationSearchResult, 0, len(payload))
	for _, item := range payload {
		lat, err := strconv.ParseFloat(item.Lat, 64)
		if err != nil {
			continue
		}
		lng, err := strconv.ParseFloat(item.Lon, 64)
		if err != nil {
			continue
		}
		results = append(results, LocationSearchResult{
			DisplayName: item.DisplayName,
			Lat:         lat,
			Lng:         lng,
		})
	}

	a.searchMu.Lock()
	a.searchCache[cacheKey] = append([]LocationSearchResult(nil), results...)
	a.searchMu.Unlock()

	return results, nil
}

// ---- 场景预设 ----

// ImportScene 导入场景预设文件
func (a *App) ImportScene() (string, error) {
	if a.sceneManager == nil {
		return "", nil
	}
	importPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || importPath == "" {
		return "", err
	}
	return a.sceneManager.Import(importPath)
}

// ExportScene 导出场景预设文件
func (a *App) ExportScene(name string) (string, error) {
	if a.sceneManager == nil {
		return "", nil
	}
	exportPath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		DefaultFilename: name + ".json",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || exportPath == "" {
		return "", err
	}
	return a.sceneManager.Export(name, exportPath)
}

// ListScenes 列出所有场景名称
func (a *App) ListScenes() ([]string, error) {
	if a.sceneManager == nil {
		return nil, nil
	}
	return a.sceneManager.List()
}

// LoadScene 加载指定场景
func (a *App) LoadScene(name string) (*config.Config, error) {
	if a.sceneManager == nil {
		return nil, nil
	}
	cfg, err := a.sceneManager.Load(name)
	if err != nil {
		return nil, err
	}
	a.setConfigBaseline(cfg, name)
	return cfg, nil
}

// SaveScene 保存场景
func (a *App) SaveScene(name string, cfg config.Config) error {
	if a.sceneManager == nil {
		return nil
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return fmt.Errorf("场景名不能为空")
	}

	validatedCfg, err := validateConfigCopy(&cfg)
	if err != nil {
		return err
	}

	exists, err := a.sceneManager.Exists(trimmedName)
	if err != nil {
		return err
	}

	if exists {
		choice, dialogErr := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
			Type:          wailsruntime.QuestionDialog,
			Title:         "Replace Preset? / 覆盖预设？",
			Message:       fmt.Sprintf("Preset \"%s\" already exists. Replace it?\n预设“%s”已存在，是否覆盖？", trimmedName, trimmedName),
			Buttons:       []string{overwriteSceneButton, closeCancelButton},
			DefaultButton: overwriteSceneButton,
			CancelButton:  closeCancelButton,
		})
		if dialogErr != nil {
			return dialogErr
		}
		if choice != overwriteSceneButton {
			return fmt.Errorf("已取消保存")
		}
	}

	return a.savePreset(trimmedName, validatedCfg)
}

// DeleteScene 删除场景
func (a *App) DeleteScene(name string) error {
	if a.sceneManager == nil {
		return nil
	}
	if err := a.sceneManager.Delete(name); err != nil {
		return err
	}
	a.clearSelectedScene(name)
	return nil
}

// RenameScene 重命名场景
func (a *App) RenameScene(oldName, newName string) error {
	if a.sceneManager == nil {
		return nil
	}
	trimmedOldName := strings.TrimSpace(oldName)
	trimmedNewName := strings.TrimSpace(newName)
	if trimmedOldName == trimmedNewName {
		return nil
	}
	if err := a.sceneManager.Rename(trimmedOldName, trimmedNewName); err != nil {
		return err
	}
	a.renameSelectedScene(trimmedOldName, trimmedNewName)
	return nil
}

// GetLocalIPs 返回本机所有非回环 IPv4 地址
func (a *App) GetLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			ips = append(ips, ip.String())
		}
	}
	return ips
}
