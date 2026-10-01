package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"hellfiresec/globalhawk/engines"
)

// App 后端，绑定到前端 (window.go.main.App.*)
type App struct {
	ctx context.Context

	censysQuery string
	censysNext  string
	censysPrev  string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 旧历史文件名迁移
	engines.MigrateHistoryFile(homeFile(engines.HistoryFilename), homeFile(".search_viewer_history.json"))
}

func homeFile(name string) string {
	// os.UserHomeDir 跨平台：macOS 为 $HOME，Windows 为 %USERPROFILE%
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, name)
}

// ---------- 配置 ----------

// GetConfig 读取 ~/config.ini，回填配置页；自动过滤旧版本写坏的控件对象值
func (a *App) GetConfig() map[string]string {
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	out := map[string]string{}
	for section, kv := range cfg {
		for k, v := range kv {
			v = strings.TrimSpace(v)
			if strings.HasPrefix(v, "<Py") {
				v = ""
			}
			out[section+"."+k] = v
		}
	}
	return out
}

// SaveConfig 全量保存配置（与原版「保存」按钮一致）
func (a *App) SaveConfig(cfg map[string]string) engines.SearchResult {
	res := engines.NewResult()
	data := map[string]map[string]string{}
	for fullkey, val := range cfg {
		parts := strings.SplitN(fullkey, ".", 2)
		if len(parts) != 2 {
			continue
		}
		if data[parts[0]] == nil {
			data[parts[0]] = map[string]string{}
		}
		data[parts[0]][parts[1]] = val
	}
	if err := engines.SaveConfig(homeFile("config.ini"), data); err != nil {
		return res.Logf("保存失败: %v", err)
	}
	return res.Logf("配置已保存到 %s", homeFile("config.ini"))
}

// ---------- 搜索历史 ----------

func (a *App) GetHistory(engine string) []string {
	return engines.LoadHistory(homeFile(engines.HistoryFilename), engine)
}

func (a *App) AddHistory(engine, text string) []string {
	return engines.AddHistory(homeFile(engines.HistoryFilename), engine, text, 30)
}

// ---------- 六个引擎 ----------

func (a *App) SearchFofa(query, size string) engines.SearchResult {
	res := engines.NewResult()
	res = res.Logf("fofa正在查询 ==> %s，请稍后...", query)
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	email := engines.CfgVal(cfg, "fofa_email", "your_email")
	key := engines.CfgVal(cfg, "fofa_key", "fofa_key")
	if key == "" {
		return res.Logf("请先配置FOFA API Key")
	}
	// FOFA 当前仅校验 key，email 可为空（实测 2026-09）
	return engines.Fofa(email, key, query, size)
}

func (a *App) SearchHunter(query, page, pageSize, isWeb string) engines.SearchResult {
	res := engines.NewResult()
	res = res.Logf("鹰图正在查询 ==> %s，请稍后...", query)
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	key := engines.CfgVal(cfg, "hunter_api", "your_api")
	if key == "" {
		return res.Logf("请先配置鹰图API")
	}
	return engines.Hunter(key, query, page, pageSize, isWeb)
}

func (a *App) SearchShodan(query, page, mode string) engines.SearchResult {
	res := engines.NewResult()
	res = res.Logf("shodan正在查询 ==> %s，速度稍慢，请耐心等待...", query)
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	key := engines.CfgVal(cfg, "shodan_api", "your_api")
	if key == "" {
		return res.Logf("请先配置shodan API")
	}
	return engines.Shodan(key, query, page, mode)
}

func (a *App) SearchQuake(query, size string) engines.SearchResult {
	res := engines.NewResult()
	query = engines.NormalizeQuakeQuery(query)
	engines.AddHistory(homeFile(engines.HistoryFilename), "quake", query, 30)
	res = res.Logf("quake正在查询 ==> %s，请稍后...", query)
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	key := engines.CfgVal(cfg, "quake_api", "your_api")
	if key == "" {
		return res.Logf("请先配置QUAKE API")
	}
	return engines.Quake(key, query, size)
}

func (a *App) SearchZoomEye(query, page string) engines.SearchResult {
	res := engines.NewResult()
	res = res.Logf("Zoomeye正在查询 ==> %s，请稍后...", query)
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	key := engines.CfgVal(cfg, "zoomeye_api", "your_zoomeye_api")
	if key == "" {
		return res.Logf("请先配置Zoomeye API")
	}
	return engines.ZoomEye(key, query, page)
}

func (a *App) SearchCensys(query, mode, size string) engines.SearchResult {
	res := engines.NewResult()
	res = res.Logf("Censys正在查询 ==> %s，请稍后...", query)
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	uid := engines.CfgVal(cfg, "censys_uid", "censys_uid")
	secret := engines.CfgVal(cfg, "censys_secret", "censys_secret")
	if uid == "" || secret == "" {
		return res.Logf("请先配置Censys API")
	}
	a.censysQuery = query
	res = engines.Censys(uid, secret, query, mode, "", size == "all")
	a.censysNext = res.Extra["next_cursor"]
	a.censysPrev = res.Extra["prev_cursor"]
	return res
}

// CensysPage Censys 游标翻页，direction: next / prev
func (a *App) CensysPage(direction string) engines.SearchResult {
	res := engines.NewResult()
	res = res.Logf("Censys正在努力的翻页中，请稍后...")
	cursor := a.censysNext
	if direction == "prev" {
		cursor = a.censysPrev
	}
	if cursor == "" {
		return res.Logf("已经翻到头了~")
	}
	cfg, _ := engines.LoadConfig(homeFile("config.ini"))
	uid := engines.CfgVal(cfg, "censys_uid", "censys_uid")
	secret := engines.CfgVal(cfg, "censys_secret", "censys_secret")
	res = engines.CensysPage(uid, secret, a.censysQuery, cursor)
	a.censysNext = res.Extra["next_cursor"]
	a.censysPrev = res.Extra["prev_cursor"]
	return res
}

// OpenURL 用系统默认浏览器打开链接
func (a *App) OpenURL(u string) {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		runtime.BrowserOpenURL(a.ctx, u)
	}
}

// ---------- 导出 ----------

// ExportCSV 弹出保存对话框，把当前表格写入用户选择的文件，返回完整路径（取消返回空串）
func (a *App) ExportCSV(prefix string, header []string, rows [][]string) string {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "选择导出位置",
		DefaultFilename:      fmt.Sprintf("%s_%s.csv", prefix, time.Now().Format("2006-01-02-15-04-05")),
		CanCreateDirectories: true,
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil || path == "" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(path), ".csv") {
		path += ".csv"
	}
	if err := engines.WriteCSV(path, header, rows); err != nil {
		return ""
	}
	// 记住导出目录，与历史记录同一个文件
	engines.RememberExportDir(homeFile(engines.HistoryFilename), filepath.Dir(path))
	return path
}
