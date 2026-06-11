package scenes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"virtual-device-ui/internal/config"
)

// Manager 场景预设管理器
type Manager struct {
	dir string
}

func normalizeSceneName(name string) string {
	return strings.TrimSpace(name)
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".virtual-device-ui", "scenes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	m := &Manager{dir: dir}
	m.seedDefaults()
	return m, nil
}

func (m *Manager) seedDefaults() {
	type defaultScene struct {
		name  string
		lat   float64
		lng   float64
		count int
	}
	defaults := []defaultScene{
		// {"鹰潭演示", 28.2378405, 117.1143221, 5},
		// {"北京演示", 39.9042, 116.4074, 5},
		// {"上海演示", 31.2304, 121.4737, 3},
	}
	for _, d := range defaults {
		path := m.path(d.name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			cfg := config.DefaultConfig()
			cfg.CenterPoint = config.GPS{Lat: d.lat, Lng: d.lng}
			cfg.Analysis.DroneCount = d.count
			cfg.Detection.DroneCount = d.count
			_ = m.Save(d.name, cfg)
		}
	}
}

func (m *Manager) path(name string) string {
	safe := strings.ReplaceAll(normalizeSceneName(name), string(filepath.Separator), "_")
	return filepath.Join(m.dir, safe+".json")
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-scene-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if retryErr := os.Rename(tmpName, path); retryErr != nil {
			return retryErr
		}
	}

	return nil
}

func ensureJSONExtension(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return path
	}
	return path + ".json"
}

func sceneNameFromImportPath(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "" {
		return "imported-scene"
	}
	return name
}

func (m *Manager) uniqueName(base string) string {
	name := normalizeSceneName(base)
	for i := 1; ; i++ {
		if _, err := os.Stat(m.path(name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s (%d)", base, i)
	}
}

// List 列出所有场景名称
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// Exists returns whether a scene name is already present.
func (m *Manager) Exists(name string) (bool, error) {
	normalizedName := normalizeSceneName(name)
	if normalizedName == "" {
		return false, fmt.Errorf("场景名不能为空")
	}

	_, err := os.Stat(m.path(normalizedName))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

// Load 加载指定场景配置
func (m *Manager) Load(name string) (*config.Config, error) {
	normalizedName := normalizeSceneName(name)
	data, err := os.ReadFile(m.path(normalizedName))
	if err != nil {
		return nil, fmt.Errorf("场景 %q 不存在", normalizedName)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := config.ValidateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("场景 %q 配置无效: %w", normalizedName, err)
	}
	return &cfg, nil
}

// Save 保存场景
func (m *Manager) Save(name string, cfg *config.Config) error {
	return m.save(name, cfg, false)
}

// Replace 显式覆盖一个已存在的场景。
func (m *Manager) Replace(name string, cfg *config.Config) error {
	return m.save(name, cfg, true)
}

func (m *Manager) save(name string, cfg *config.Config, overwrite bool) error {
	normalizedName := normalizeSceneName(name)
	if normalizedName == "" {
		return fmt.Errorf("场景名不能为空")
	}
	if cfg == nil {
		return fmt.Errorf("场景配置不能为空")
	}
	if err := config.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("场景配置无效: %w", err)
	}

	exists, err := m.Exists(normalizedName)
	if err != nil {
		return err
	}
	if exists && !overwrite {
		return fmt.Errorf("场景 %q 已存在", normalizedName)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(m.path(normalizedName), data)
}

// Export 导出场景到指定文件
func (m *Manager) Export(name, exportPath string) (string, error) {
	normalizedName := normalizeSceneName(name)
	data, err := os.ReadFile(m.path(normalizedName))
	if err != nil {
		return "", fmt.Errorf("场景 %q 不存在", normalizedName)
	}

	targetPath := ensureJSONExtension(exportPath)
	if err := writeFileAtomic(targetPath, data); err != nil {
		return "", err
	}
	return targetPath, nil
}

// Import 从文件导入场景，若同名则自动追加序号
func (m *Manager) Import(importPath string) (string, error) {
	data, err := os.ReadFile(importPath)
	if err != nil {
		return "", err
	}

	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("场景文件格式无效: %w", err)
	}
	if err := config.ValidateConfig(&cfg); err != nil {
		return "", fmt.Errorf("场景文件配置无效: %w", err)
	}

	name := m.uniqueName(sceneNameFromImportPath(importPath))
	if err := m.Save(name, &cfg); err != nil {
		return "", err
	}
	return name, nil
}

// Delete 删除场景
func (m *Manager) Delete(name string) error {
	normalizedName := normalizeSceneName(name)
	if normalizedName == "" {
		return fmt.Errorf("场景名不能为空")
	}
	if err := os.Remove(m.path(normalizedName)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("场景 %q 不存在", normalizedName)
		}
		return err
	}
	return nil
}

// Rename 重命名场景
func (m *Manager) Rename(oldName, newName string) error {
	normalizedOldName := normalizeSceneName(oldName)
	normalizedNewName := normalizeSceneName(newName)

	if normalizedOldName == "" {
		return fmt.Errorf("原场景名不能为空")
	}
	if normalizedNewName == "" {
		return fmt.Errorf("新场景名不能为空")
	}
	if normalizedOldName == normalizedNewName {
		return nil
	}

	exists, err := m.Exists(normalizedOldName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("场景 %q 不存在", normalizedOldName)
	}

	targetExists, err := m.Exists(normalizedNewName)
	if err != nil {
		return err
	}
	if targetExists {
		return fmt.Errorf("场景 %q 已存在", normalizedNewName)
	}

	return os.Rename(m.path(normalizedOldName), m.path(normalizedNewName))
}
