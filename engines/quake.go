package engines

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const quakeSearchAPI = "https://quake.360.net/api/v3/search/quake_service"

var (
	quakeIPOrCIDR = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}(/\d{1,2})?$`)
	quakeDomain   = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+$`)
)

// NormalizeQuakeQuery Quake 对裸字符串不做字段过滤（会返回全量无关数据），
// 没写语法时自动转换：裸IP -> ip:"..."，裸域名 -> domain:"..."
func NormalizeQuakeQuery(search string) string {
	s := strings.TrimSpace(search)
	if s == "" || strings.ContainsAny(s, `:"<>&|=()`) {
		return s
	}
	if quakeIPOrCIDR.MatchString(s) {
		return fmt.Sprintf("ip:\"%s\"", s)
	}
	if quakeDomain.MatchString(s) {
		return fmt.Sprintf("domain:\"%s\"", s)
	}
	return s
}

// Quake 360 Quake 服务搜索 + 本月剩余积分
// size == "all" 时自动翻页拉取全部结果（每页 500，上限 5000 条，按实际拉取量消耗积分）
func Quake(key, query, size string) SearchResult {
	res := NewResult()
	headers := map[string]string{
		"X-QuakeToken": key,
		"Content-Type": "application/json",
	}

	// 先独立刷新剩余积分，查询失败也能看到最新值
	var info struct {
		Code int `json:"code"`
		Data struct {
			MonthRemainingCredit int `json:"month_remaining_credit"`
		} `json:"data"`
	}
	if err := doJSON("GET", "https://quake.360.net/api/v3/user/info", headers, nil, &info); err != nil {
		res.Extra["credit"] = "获取失败"
		res.Logf("剩余积分获取失败，请检查Quake API Key")
	} else {
		res.Extra["credit"] = fmt.Sprintf("本月剩余积分：%d", info.Data.MonthRemainingCredit)
	}

	var items []map[string]interface{}
	total := -1
	if size == "all" {
		items, total = quakeFetchAll(key, query, headers, &res)
	} else {
		var err bool
		items, total, err = quakeSinglePage(key, query, jsonSafeInt(size, "100"), &res)
		if err {
			return res
		}
	}

	if total < 0 {
		total = len(items)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	res.Logf("已找到：%d 条结果", total)
	for _, item := range items {
		service := mapMap(item, "service")
		httpInfo := mapMap(service, "http")
		res.Rows = append(res.Rows, []string{
			mapString(item, "ip"),
			mapString(item, "port"),
			mapString(httpInfo, "title"),
			mapString(httpInfo, "status_code"),
			mapString(mapMap(item, "location"), "province_en"),
			mapString(item, "org"),
			ResolveQuakeHostname(item),
			BuildQuakeURL(item),
		})
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
	return res
}

// quakeSinglePage 单页查询，err 为 true 表示已写入失败日志
func quakeSinglePage(key, query, size string, res *SearchResult) (items []map[string]interface{}, total int, err bool) {
	headers := map[string]string{
		"X-QuakeToken": key,
		"Content-Type": "application/json",
	}
	body := fmt.Sprintf(
		`{"query":%q,"start":0,"size":%s,"ignore_cache":false,"start_time":"2000-12-01 00:00:00","end_time":"2090-12-02 00:00:00"}`,
		query, size)
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Meta    struct {
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		} `json:"meta"`
		Data []map[string]interface{} `json:"data"`
	}
	if err := doJSON("POST", quakeSearchAPI, headers, []byte(body), &resp); err != nil {
		res.Logf("查询出错，请检查网络和API配置")
		return nil, -1, true
	}
	switch resp.Code {
	case 0:
	case 3007:
		res.Logf("积分不足,无法查询")
		return nil, -1, true
	case 3005:
		res.Logf("调用API过于频繁")
		return nil, -1, true
	case 3017:
		res.Logf("暂不支持该字段查询")
		return nil, -1, true
	default:
		res.Logf("查询出错: %s", resp.Message)
		return nil, -1, true
	}
	return resp.Data, resp.Meta.Pagination.Total, false
}

// quakeFetchAll 自动翻页拉取全部结果；中途积分不足/频控/网络异常时保留已获取部分
func quakeFetchAll(key, query string, headers map[string]string, res *SearchResult) ([]map[string]interface{}, int) {
	const pageSize = 500
	const hardCap = 5000
	var collected []map[string]interface{}
	total := -1

	for start := 0; ; start += pageSize {
		body := fmt.Sprintf(
			`{"query":%q,"start":%d,"size":%d,"ignore_cache":false,"start_time":"2000-12-01 00:00:00","end_time":"2090-12-02 00:00:00"}`,
			query, start, pageSize)
		var resp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Meta    struct {
				Pagination struct {
					Total int `json:"total"`
				} `json:"pagination"`
			} `json:"meta"`
			Data []map[string]interface{} `json:"data"`
		}
		if err := doJSON("POST", quakeSearchAPI, headers, []byte(body), &resp); err != nil {
			if len(collected) > 0 {
				res.Logf("网络异常，已获取 %d 条后停止", len(collected))
			} else {
				res.Logf("查询出错，请检查网络和API配置")
			}
			break
		}
		if resp.Code != 0 {
			reason := fmt.Sprintf("接口返回 %d（%s）", resp.Code, resp.Message)
			if resp.Code == 3007 {
				reason = "积分不足"
			} else if resp.Code == 3005 {
				reason = "调用API过于频繁"
			}
			if len(collected) > 0 {
				res.Logf("%s，已获取 %d 条后停止", reason, len(collected))
			} else {
				res.Logf("%s，无法查询", reason)
			}
			break
		}
		if total < 0 {
			total = resp.Meta.Pagination.Total
			res.Logf("已找到：%d 条结果，自动翻页拉取中（每页 %d 条）...", total, pageSize)
		}
		collected = append(collected, resp.Data...)
		res.Logf("已获取 %d / %d 条", min(len(collected), total), total)
		if len(resp.Data) == 0 || len(collected) >= total || len(collected) >= hardCap {
			break
		}
		time.Sleep(300 * time.Millisecond) // 防频控
	}
	if total < 0 {
		total = len(collected)
	}
	return collected, total
}

// jsonSafeInt 下拉框传来的数字字符串，非法时用默认值
func jsonSafeInt(s, def string) string {
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
	}
	if s == "" {
		return def
	}
	return s
}

// ResolveQuakeHostname Quake 的 hostname 是 rDNS（反向DNS），多数资产为空；
// 依次回退：rDNS -> 网站域名 domain -> http host
func ResolveQuakeHostname(item map[string]interface{}) string {
	host := mapString(item, "hostname")
	if host == "" {
		host = mapString(item, "domain")
	}
	if host == "" {
		host = mapString(mapMap(mapMap(item, "service"), "http"), "host")
	}
	return host
}

// BuildQuakeURL web 服务拼出可访问的完整 URL，非 web 服务（如 rtsp/ssh）留空
func BuildQuakeURL(item map[string]interface{}) string {
	service := mapMap(item, "service")
	svcName := strings.ToLower(mapString(service, "name"))
	if !strings.Contains(svcName, "http") {
		return ""
	}
	ip := mapString(item, "ip")
	hostPart := mapString(mapMap(service, "http"), "host")
	if hostPart == "" {
		hostPart = mapString(item, "hostname")
	}
	if hostPart == "" {
		hostPart = mapString(item, "domain")
	}
	if hostPart == "" {
		hostPart = ip
	}
	if hostPart == ip && strings.Contains(ip, ":") { // IPv6 裸地址
		hostPart = fmt.Sprintf("[%s]", ip)
	}
	port := mapString(item, "port")
	scheme := "http"
	if strings.Contains(svcName, "ssl") || strings.Contains(svcName, "https") {
		scheme = "https"
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return fmt.Sprintf("%s://%s", scheme, hostPart)
	}
	return fmt.Sprintf("%s://%s:%s", scheme, hostPart, port)
}
