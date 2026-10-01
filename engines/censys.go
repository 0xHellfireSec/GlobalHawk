package engines

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var censysIPPattern = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(/\d{1,2})?$`)

func censysAuth(uid, secret string) map[string]string {
	return map[string]string{"Authorization": "Basic " + base64Encode(uid+":"+secret)}
}

// Censys Censys 主机搜索 / 单 IP 详情
// fetchAll == true 时自动沿 cursor 翻页拉取全部（每页 100，上限 2000 条）
func Censys(uid, secret, query, mode, cursor string, fetchAll bool) SearchResult {
	res := NewResult()
	auth := censysAuth(uid, secret)

	if mode == "IP" && !fetchAll && cursor == "" && censysIPPattern.MatchString(strings.TrimSpace(query)) {
		return censysHostDetail(auth, strings.TrimSpace(query))
	}
	if fetchAll {
		return censysFetchAll(auth, query, &res)
	}

	hits, total, next, prev, fail := censysSearchRaw(auth, query, cursor)
	if fail != "" {
		return res.Logf("%s", fail)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	res.Extra["next_cursor"] = next
	res.Extra["prev_cursor"] = prev
	buildCensysRows(&res, hits, total)
	return res
}

// CensysPage Censys 游标翻页，cursor 为空表示已到头
func CensysPage(uid, secret, query, cursor string) SearchResult {
	res := NewResult()
	if cursor == "" {
		return res.Logf("已经翻到头了~")
	}
	auth := censysAuth(uid, secret)
	hits, total, next, prev, fail := censysSearchRaw(auth, query, cursor)
	if fail != "" {
		return res.Logf("%s", fail)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	res.Extra["next_cursor"] = next
	res.Extra["prev_cursor"] = prev
	buildCensysRows(&res, hits, total)
	return res
}

// censysFetchAll 自动沿 cursor 翻页拉取全部；中途失败保留已获取部分
func censysFetchAll(auth map[string]string, query string, res *SearchResult) SearchResult {
	const hardCap = 2000
	var collected []map[string]interface{}
	total := -1
	cursor := ""

	for {
		hits, t, next, _, fail := censysSearchRaw(auth, query, cursor)
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
		collected = append(collected, hits...)
		if total > 0 {
			res.Logf("已获取 %d / %d 条", min(len(collected), total), total)
		} else {
			res.Logf("已获取 %d 条", len(collected))
		}
		if next == "" || len(hits) == 0 || (total > 0 && len(collected) >= total) || len(collected) >= hardCap {
			break
		}
		cursor = next
		time.Sleep(300 * time.Millisecond) // 防频控
	}
	if total < 0 {
		total = len(collected)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	buildCensysRows(res, collected, total)
	return *res
}

// censysSearchRaw 请求搜索接口，返回原始 hits/total/cursor；fail 非空表示失败
func censysSearchRaw(auth map[string]string, query, cursor string) (hits []map[string]interface{}, total int, next, prev, fail string) {
	api := fmt.Sprintf("https://search.censys.io/api/v2/hosts/search?q=%s&per_page=100&cursor=%s",
		url.QueryEscape(query), url.QueryEscape(cursor))
	var resp struct {
		Code  int `json:"code"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Total int                      `json:"total"`
			Hits  []map[string]interface{} `json:"hits"`
			Links struct {
				Next string `json:"next"`
				Prev string `json:"prev"`
			} `json:"links"`
		} `json:"result"`
	}
	if err := doJSON("GET", api, auth, nil, &resp); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "403") || strings.Contains(msg, "401"):
			return nil, -1, "", "", "API 错误，请检查Censys 账号"
		case strings.Contains(msg, "429"):
			return nil, -1, "", "", "请求太多,您已使用了此计费期间的全部配额"
		}
		return nil, -1, "", "", "请检查语法是否正确！！！"
	}
	if resp.Code != 200 && resp.Result.Total == 0 && resp.Error.Message != "" {
		return nil, -1, "", "", "请检查语法是否正确！！！"
	}
	return resp.Result.Hits, resp.Result.Total, resp.Result.Links.Next, resp.Result.Links.Prev, ""
}

// buildCensysRows Hosts 模式数据行 + 收尾日志
func buildCensysRows(res *SearchResult, hits []map[string]interface{}, total int) {
	if total > 0 {
		res.Logf("已找到：%d 条结果", total)
	}
	for _, hit := range hits {
		ports, services := []string{}, []string{}
		for _, svc := range toSlice(hit["services"]) {
			m := svc.(map[string]interface{})
			if p := mapString(m, "port"); p != "" {
				ports = append(ports, p)
				services = append(services, mapString(m, "extended_service_name"))
			}
		}
		if len(ports) == 0 {
			ports, services = []string{"-"}, []string{"-"}
		}
		res.Rows = append(res.Rows, []string{
			mapString(hit, "ip"),
			strings.Join(ports, ","),
			strings.Join(services, ","),
			mapString(mapMap(hit, "autonomous_system"), "name"),
			mapString(mapMap(hit, "location"), "country"),
		})
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
}

// censysHostDetail 单 IP 详情
func censysHostDetail(auth map[string]string, ip string) SearchResult {
	res := NewResult()
	api := fmt.Sprintf("https://search.censys.io/api/v2/hosts/%s", ip)
	var resp struct {
		Code   int `json:"code"`
		Result struct {
			IP       string                   `json:"ip"`
			Location map[string]interface{}   `json:"location"`
			Services []map[string]interface{} `json:"services"`
		} `json:"result"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := doJSON("GET", api, auth, nil, &resp); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "403") || strings.Contains(msg, "401"):
			return res.Logf("API 错误")
		case strings.Contains(msg, "429"):
			return res.Logf("请求太多,您已使用了此计费期间的全部配额")
		}
		return res.Logf("未知错误,请排查")
	}
	if resp.Code != 200 {
		if resp.Error.Message != "" {
			return res.Logf("查询失败: %s", resp.Error.Message)
		}
		return res.Logf("未知错误,请排查")
	}

	country := mapString(resp.Result.Location, "country")
	for _, svc := range resp.Result.Services {
		title := ""
		method := ""
		statusCode := ""
		if httpInfo := mapMap(svc, "http"); httpInfo != nil {
			method = mapString(mapMap(httpInfo, "request"), "method")
			statusCode = mapString(mapMap(httpInfo, "response"), "status_code")
			title = mapString(mapMap(httpInfo, "response"), "html_title")
		}
		domain := ""
		if tlsInfo := mapMap(svc, "tls"); tlsInfo != nil {
			domain = mapStringPath(tlsInfo, "certificates", "leaf_data", "subject_dn")
		}
		res.Rows = append(res.Rows, []string{
			resp.Result.IP,
			domain,
			mapString(svc, "port"),
			title,
			mapString(svc, "extended_service_name"),
			country,
			method,
			statusCode,
		})
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
	return res
}

// toSlice interface{} -> []interface{}
func toSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}
