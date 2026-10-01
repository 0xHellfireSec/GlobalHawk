package engines

import (
	"fmt"
	"net/url"
	"time"
)

// Shodan SHODAN 搜索，mode: HOST / IP
// page == "all" 且 mode 为 HOST 时自动翻页拉取全部（每页 100，上限 2000 条）
func Shodan(key, query, page, mode string) SearchResult {
	res := NewResult()

	if mode == "IP" {
		info := fmt.Sprintf("https://api.shodan.io/shodan/host/%s?key=%s", url.PathEscape(query), key)
		var resp map[string]interface{}
		if err := doJSON("GET", info, nil, nil, &resp); err != nil {
			return res.Logf("shodan提示：%v", err)
		}
		if msg := mapString(resp, "error"); msg != "" {
			return res.Logf("shodan提示：%s", msg)
		}
		row := []string{
			mapString(resp, "ip_str"),
			toString(resp["ports"]),
			toString(resp["domains"]),
			mapString(resp, "os"),
			mapString(mapMap(resp, "location"), "country_name"),
			mapString(resp, "org"),
			mapString(resp, "isp"),
			mapString(resp, "last_update"),
		}
		res.Rows = append(res.Rows, row)
		res.Logf("查询状态：完成")
		res.Log = append(res.Log, "-----------------------------")
		return res
	}

	if page == "all" {
		return shodanFetchAll(key, query, &res)
	}

	search := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=%s&page=%s",
		key, url.QueryEscape(query), page)
	var resp struct {
		Total   int                      `json:"total"`
		Matches []map[string]interface{} `json:"matches"`
		Error   string                   `json:"error"`
	}
	if err := doJSON("GET", search, nil, nil, &resp); err != nil {
		return res.Logf("shodan提示：%v", err)
	}
	if resp.Error != "" {
		return res.Logf("shodan提示：%s", resp.Error)
	}

	res.Extra["count"] = fmt.Sprintf("%d", resp.Total)
	res.Logf("已找到：%d 条结果", resp.Total)
	for _, item := range resp.Matches {
		res.Rows = append(res.Rows, []string{
			mapString(item, "ip_str"),
			mapString(item, "port"),
			toString(item["domains"]),
			mapString(item, "os"),
			mapString(mapMap(item, "location"), "country_name"),
			mapString(item, "org"),
			mapString(item, "isp"),
			mapString(item, "timestamp"),
		})
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
	return res
}

// shodanFetchAll 自动翻页拉取全部结果；中途失败保留已获取部分
func shodanFetchAll(key, query string, res *SearchResult) SearchResult {
	const perPage = 100
	const hardCap = 2000
	var collected []map[string]interface{}
	total := -1

	for p := 1; ; p++ {
		search := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=%s&page=%d",
			key, url.QueryEscape(query), p)
		var resp struct {
			Total   int                      `json:"total"`
			Matches []map[string]interface{} `json:"matches"`
			Error   string                   `json:"error"`
		}
		if err := doJSON("GET", search, nil, nil, &resp); err != nil {
			if len(collected) > 0 {
				res.Logf("shodan提示：%v，已获取 %d 条后停止", err, len(collected))
			} else {
				return res.Logf("shodan提示：%v", err)
			}
			break
		}
		if resp.Error != "" {
			if len(collected) > 0 {
				res.Logf("shodan提示：%s，已获取 %d 条后停止", resp.Error, len(collected))
			} else {
				return res.Logf("shodan提示：%s", resp.Error)
			}
			break
		}
		if total < 0 {
			total = resp.Total
			res.Logf("已找到：%d 条结果，自动翻页拉取中（每页 %d 条）...", total, perPage)
		}
		collected = append(collected, resp.Matches...)
		res.Logf("已获取 %d / %d 条", min(len(collected), max(total, 0)), total)
		if len(resp.Matches) == 0 || len(resp.Matches) < perPage ||
			(total > 0 && len(collected) >= total) || len(collected) >= hardCap {
			break
		}
		time.Sleep(300 * time.Millisecond) // 防频控
	}
	if total < 0 {
		total = len(collected)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	res.Logf("已找到：%d 条结果", total)
	for _, item := range collected {
		res.Rows = append(res.Rows, []string{
			mapString(item, "ip_str"),
			mapString(item, "port"),
			toString(item["domains"]),
			mapString(item, "os"),
			mapString(mapMap(item, "location"), "country_name"),
			mapString(item, "org"),
			mapString(item, "isp"),
			mapString(item, "timestamp"),
		})
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
	return *res
}
