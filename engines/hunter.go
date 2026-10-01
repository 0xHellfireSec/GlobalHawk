package engines

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// defaultUA 与 Python 版一致的浏览器 User-Agent（Hunter 等 WAF 会拦 Go 默认 UA）
const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:81.0) Gecko/20100101 Firefox/81.0"

// Hunter 奇安信鹰图
// pageSize == "all" 时自动翻页拉取全部（每页 100，上限 5000 条，按实际消耗积分）
func Hunter(key, query, page, pageSize, isWeb string) SearchResult {
	res := NewResult()
	if pageSize == "all" {
		return hunterFetchAll(key, query, isWeb, &res)
	}

	data, total, consume, rest, fail := hunterPage(key, query, page, pageSize, isWeb)
	if fail != "" {
		return res.Logf("%s", fail)
	}
	if consume != "" {
		res.Extra["consume"] = consume
	}
	if rest != "" {
		res.Extra["rest_quota"] = rest
	}
	buildHunterRows(&res, data, total)
	return res
}

// hunterFetchAll 自动翻页拉取全部结果；中途失败保留已获取部分
func hunterFetchAll(key, query, isWeb string, res *SearchResult) SearchResult {
	const perPage = 100
	const hardCap = 5000
	var collected []map[string]interface{}
	total := -1

	for p := 1; ; p++ {
		data, t, _, rest, fail := hunterPage(key, query, fmt.Sprint(p), fmt.Sprint(perPage), isWeb)
		if fail != "" {
			if len(collected) > 0 {
				res.Logf("%s，已获取 %d 条后停止", fail, len(collected))
			} else {
				res.Logf("%s", fail)
			}
			break
		}
		if rest != "" {
			res.Extra["rest_quota"] = rest
		}
		if t >= 0 {
			total = t
		}
		collected = append(collected, data...)
		if total > 0 {
			res.Logf("已获取 %d / %d 条", min(len(collected), total), total)
		} else {
			res.Logf("已获取 %d 条", len(collected))
		}
		if len(data) == 0 || (total > 0 && len(collected) >= total) || len(collected) >= hardCap {
			break
		}
		time.Sleep(300 * time.Millisecond) // 防频控
	}
	if total < 0 {
		total = len(collected)
	}
	res.Extra["count"] = fmt.Sprintf("%d", total)
	buildHunterRows(res, collected, total)
	return *res
}

// buildHunterRows 数据行 + 收尾日志
func buildHunterRows(res *SearchResult, data []map[string]interface{}, total int) {
	if total == 0 {
		res.Logf("鹰图提示：未找到记录")
	}
	if total > 0 {
		res.Logf("已找到：%d 条结果", total)
	}
	for _, item := range data {
		res.Rows = append(res.Rows, rowOf(item,
			"url", "ip", "port", "web_title", "domain", "protocol",
			"status_code", "company", "country", "updated_at", "isp"))
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
}

// hunterPage 请求单页，返回数据/总数/积分信息；fail 非空表示失败（已含友好文案）
func hunterPage(key, query, page, pageSize, isWeb string) (data []map[string]interface{}, total int, consume, rest, fail string) {
	search := base64.URLEncoding.EncodeToString([]byte(query))

	params := url.Values{}
	params.Set("api-key", key)
	params.Set("search", search)
	params.Set("page", page)
	params.Set("page_size", pageSize)
	params.Set("is_web", isWeb)
	params.Set("start_time", `"2000-01-01 00:00:00"`)
	params.Set("end_time", `"2030-03-01 00:00:00"`)
	api := "https://hunter.qianxin.com/openApi/search?" + params.Encode()

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total        int                      `json:"total"`
			ConsumeQuota interface{}              `json:"consume_quota"`
			RestQuota    interface{}              `json:"rest_quota"`
			Arr          []map[string]interface{} `json:"arr"`
		} `json:"data"`
	}
	if err := doJSON("GET", api, map[string]string{"User-Agent": defaultUA}, nil, &resp); err != nil {
		return nil, -1, "", "", fmt.Sprintf("查询出错: %v", err)
	}

	if strings.Contains(resp.Message, "积分用完了") {
		return nil, -1, "", "", "大牛，您的积分用完了，明天再试试"
	} else if strings.Contains(resp.Message, "令牌过期") {
		return nil, -1, "", "", "API 信息有误"
	} else if strings.Contains(resp.Message, "令牌缺失") {
		return nil, -1, "", "", "请先配置 API"
	} else if strings.Contains(resp.Message, "存在违规字符") {
		return nil, -1, "", "", "存在违规字符"
	} else if strings.Contains(resp.Message, "请求太多") {
		return nil, -1, "", "", "请求太多啦，稍后再试试"
	}
	if resp.Code == 400 {
		return nil, -1, "", "", "语法有误，请排查"
	}

	if resp.Data.Total == 0 {
		return nil, 0, toString(resp.Data.ConsumeQuota), toString(resp.Data.RestQuota), ""
	}
	return resp.Data.Arr, resp.Data.Total, toString(resp.Data.ConsumeQuota), toString(resp.Data.RestQuota), ""
}
