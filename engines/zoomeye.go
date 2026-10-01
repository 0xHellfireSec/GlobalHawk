package engines

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ZoomEye 钟馗之眼 主机搜索 + 本月剩余额度
// page == "all" 时自动翻页拉取全部（每页 20，上限 2000 条）
func ZoomEye(key, query, page string) SearchResult {
	res := NewResult()
	headers := map[string]string{"API-KEY": key}

	// 先独立刷新剩余额度
	var resInfo struct {
		QuotaInfo struct {
			RemainTotalQuota int `json:"remain_total_quota"`
		} `json:"quota_info"`
	}
	if err := doJSON("GET", "https://api.zoomeye.org/resources-info", headers, nil, &resInfo); err != nil {
		res.Extra["quota"] = "获取失败"
		res.Logf("剩余积分获取失败，请检查Zoomeye API Key")
	} else {
		res.Extra["quota"] = fmt.Sprintf("本月剩余积分：%d", resInfo.QuotaInfo.RemainTotalQuota)
	}

	var matches []map[string]interface{}
	total := -1
	if page == "all" {
		matches, total = zoomeyeFetchAll(key, query, headers, &res)
	} else {
		pageMatches, t, end, fail := zoomeyePage(key, query, page, headers)
		if fail != "" {
			return res.Logf("%s", fail)
		}
		if end {
			res.Logf("没有下一页了,请返回上一页")
			return res
		}
		matches, total = pageMatches, t
	}

	if total < 0 {
		total = len(matches)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	res.Logf("已找到：%d 条结果", total)
	buildZoomEyeRows(&res, matches)
	return res
}

func buildZoomEyeRows(res *SearchResult, matches []map[string]interface{}) {
	for _, match := range matches {
		portinfo := mapMap(match, "portinfo")
		res.Rows = append(res.Rows, []string{
			mapString(match, "ip"),
			mapString(portinfo, "port"),
			mapString(portinfo, "extrainfo"),
			mapStringPath(match, "geoinfo", "city", "names", "en"),
			mapString(portinfo, "app"),
			mapString(mapMap(match, "geoinfo"), "organization"),
			mapString(match, "timestamp"),
			BuildZoomEyeURL(match),
		})
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
}

// zoomeyePage 请求单页；end=true 表示已到末尾（50005）
func zoomeyePage(key, query, page string, headers map[string]string) (matches []map[string]interface{}, total int, end bool, fail string) {
	search := fmt.Sprintf("https://api.zoomeye.org/host/search?query=%s&page=%s",
		url.QueryEscape(query), page)
	var resp struct {
		Code    int                      `json:"code"`
		Total   int                      `json:"total"`
		Matches []map[string]interface{} `json:"matches"`
	}
	if err := doJSON("GET", search, headers, nil, &resp); err != nil {
		return nil, -1, false, "Zoomeye 返回异常，请检查API Key"
	}
	switch resp.Code {
	case 60000:
		return resp.Matches, resp.Total, false, ""
	case 50003:
		return nil, -1, false, "IP地址查询今天达到最大限制"
	case 50005:
		return nil, -1, true, ""
	default:
		return nil, -1, false, "查询失败，请检查API Key和查询语法"
	}
}

// zoomeyeFetchAll 自动翻页拉取全部结果；中途失败保留已获取部分
func zoomeyeFetchAll(key, query string, headers map[string]string, res *SearchResult) ([]map[string]interface{}, int) {
	const hardCap = 2000
	var collected []map[string]interface{}
	total := -1

	for p := 1; ; p++ {
		matches, t, end, fail := zoomeyePage(key, query, fmt.Sprint(p), headers)
		if fail != "" {
			if len(collected) > 0 {
				res.Logf("%s，已获取 %d 条后停止", fail, len(collected))
			} else {
				res.Logf("%s", fail)
			}
			break
		}
		if t >= 0 {
			total = t
		}
		collected = append(collected, matches...)
		if total > 0 {
			res.Logf("已获取 %d / %d 条", min(len(collected), total), total)
		} else {
			res.Logf("已获取 %d 条", len(collected))
		}
		if end || len(matches) == 0 || (total > 0 && len(collected) >= total) || len(collected) >= hardCap {
			break
		}
		time.Sleep(300 * time.Millisecond) // 防频控
	}
	if total < 0 {
		total = len(collected)
	}
	return collected, total
}

// BuildZoomEyeURL web 服务拼出可访问的完整 URL，非 web 服务（如 ssh/redis）留空
func BuildZoomEyeURL(match map[string]interface{}) string {
	portinfo := mapMap(match, "portinfo")
	svc := strings.ToLower(mapString(portinfo, "service"))
	if !strings.Contains(svc, "http") {
		return ""
	}
	port := mapString(portinfo, "port")
	ip := mapString(match, "ip")
	if strings.Contains(ip, ":") {
		ip = fmt.Sprintf("[%s]", ip)
	}
	scheme := "http"
	if strings.Contains(svc, "https") || port == "443" {
		scheme = "https"
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return fmt.Sprintf("%s://%s", scheme, ip)
	}
	return fmt.Sprintf("%s://%s:%s", scheme, ip, port)
}
