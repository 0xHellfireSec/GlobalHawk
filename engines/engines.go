// Package engines 实现六个网络空间测绘引擎的 API 客户端与公共设施。
package engines

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// base64Encode 标准 Base64
func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// HistoryFilename 搜索历史/导出目录记录文件（位于用户主目录）
const HistoryFilename = "globalhawk_history.json"

// MigrateHistoryFile 兼容旧文件名：首次使用时把旧历史文件内容迁移到新文件（旧文件保留）
func MigrateHistoryFile(path, legacyPath string) {
	if _, err := os.Stat(path); err == nil {
		return // 新文件已存在
	}
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return // 旧文件也不存在
	}
	_ = os.WriteFile(path, data, 0o644)
}

// doJSON 发起 HTTP 请求并解析为 JSON，resp 必须传指针
func doJSON(method, url string, headers map[string]string, body []byte, resp interface{}) error {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	httpResp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	decoder := json.NewDecoder(httpResp.Body)
	if err := decoder.Decode(resp); err != nil {
		return fmt.Errorf("响应解析失败(HTTP %d): %w", httpResp.StatusCode, err)
	}
	return nil
}

// toString 任意 JSON 值转字符串（nil / 缺失返回空串，数组用逗号连接）
func toString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, toString(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", t)
	}
}

// mapString 取 map 中的字符串值
func mapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return toString(m[key])
}

// mapMap 取嵌套 map
func mapMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	sub, _ := m[key].(map[string]interface{})
	return sub
}

// mapStringPath 按路径逐层取字符串值，如 mapStringPath(m, "geoinfo", "city", "names", "en")
func mapStringPath(m map[string]interface{}, keys ...string) string {
	cur := m
	for i, k := range keys {
		if cur == nil {
			return ""
		}
		if i == len(keys)-1 {
			return mapString(cur, k)
		}
		cur = mapMap(cur, k)
	}
	return ""
}

// rowOf 按字段名顺序把 map 拼成表格行
func rowOf(m map[string]interface{}, fields ...string) []string {
	row := make([]string, 0, len(fields))
	for _, f := range fields {
		row = append(row, mapString(m, f))
	}
	return row
}

// ---------- 配置（与原版 config.ini / ConfigObj 格式互通） ----------

// LoadConfig 读取 INI：[section] key = value；值两侧成对引号会被剥掉
func LoadConfig(path string) (map[string]map[string]string, error) {
	cfg := map[string]map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if cfg[section] == nil {
				cfg[section] = map[string]string{}
			}
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 || section == "" {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 && (val[0] == '\'' || val[0] == '"') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		cfg[section][key] = val
	}
	return cfg, nil
}

// SaveConfig 全量写回 INI
func SaveConfig(path string, cfg map[string]map[string]string) error {
	var b strings.Builder
	for section, kv := range cfg {
		fmt.Fprintf(&b, "[%s]\n", section)
		for k, v := range kv {
			fmt.Fprintf(&b, "%s = %s\n", k, v)
		}
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// CfgVal 取配置值，空安全
func CfgVal(cfg map[string]map[string]string, section, key string) string {
	if cfg == nil || cfg[section] == nil {
		return ""
	}
	return strings.TrimSpace(cfg[section][key])
}

// ---------- 搜索历史 / 导出目录记忆（与 Python 版同一 JSON 文件互通） ----------

type historyFile struct {
	Data map[string]interface{} `json:"data"`
}

func loadHistoryRaw(path string) map[string]interface{} {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}

func saveHistoryRaw(path string, data map[string]interface{}) error {
	b, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadHistory 读取某引擎的历史关键词（最新在前）
func LoadHistory(path, engine string) []string {
	data := loadHistoryRaw(path)
	list, _ := data[engine].([]interface{})
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// AddHistory 记录关键词：去重、置顶、截断
func AddHistory(path, engine, text string, limit int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return LoadHistory(path, engine)
	}
	hist := LoadHistory(path, engine)
	filtered := hist[:0]
	for _, v := range hist {
		if v != text {
			filtered = append(filtered, v)
		}
	}
	hist = append([]string{text}, filtered...)
	if len(hist) > limit {
		hist = hist[:limit]
	}
	data := loadHistoryRaw(path)
	data[engine] = hist
	_ = saveHistoryRaw(path, data)
	return hist
}

// RememberExportDir 记录上次导出目录
func RememberExportDir(path, dir string) {
	if dir == "" {
		return
	}
	data := loadHistoryRaw(path)
	data["export_dir"] = dir
	_ = saveHistoryRaw(path, data)
}

// ---------- CSV 导出 ----------

// WriteCSV 写 UTF-8 CSV（带 BOM，Excel 打开中文不乱码）
func WriteCSV(path string, header []string, rows [][]string) error {
	var b bytes.Buffer
	b.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&b)
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			continue
		}
	}
	w.Flush()
	return os.WriteFile(path, b.Bytes(), 0o644)
}
