package engines

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Fofa FOFA 全量搜索
// GET /api/v1/search/all?email=&key=&qbase64=&size=&fields=host,ip,port,title,domain,country,protocol,server
func Fofa(email, key, query, size string) SearchResult {
	res := NewResult()
	if size == "all" {
		// FOFA 接口单次最多返回 1 万条（账号本身限制），「全部」即 1 万
		size = "10000"
		res.Logf("FOFA 单次最多返回 1 万条，已按上限拉取")
	}
	qbase64 := base64.StdEncoding.EncodeToString([]byte(query))
	url := fmt.Sprintf(
		"https://fofa.info/api/v1/search/all?email=%s&key=%s&qbase64=%s&size=%s&fields=host,ip,port,title,domain,country,protocol,server",
		email, key, qbase64, size)

	var resp struct {
		Error   bool            `json:"error"`
		Errmsg  string          `json:"errmsg"`
		Size    int             `json:"size"`
		Results [][]interface{} `json:"results"`
	}
	if err := doJSON("GET", url, nil, nil, &resp); err != nil {
		return res.Logf("查询出错: %v", err)
	}

	if resp.Error || resp.Errmsg != "" {
		msg := resp.Errmsg
		switch {
		case contains(msg, "账号无效", "Account Invalid"):
			return res.Logf("API信息有误")
		case contains(msg, "参数错误", "Params Error"):
			return res.Logf("搜索内容不能为空")
		case contains(msg, "F币余额不足", "F Coins Insufficient Balance"):
			return res.Logf("F币余额不足，注册会员请直接使用fofa官网查询")
		case contains(msg, "45022"):
			return res.Logf("今日请求次数已达上限，继续调取将扣除F点")
		case contains(msg, "820031"):
			return res.Logf("F点余额不足")
		default:
			return res.Logf("查询出错: %s", msg)
		}
	}

	res.Extra["count"] = fmt.Sprintf("%d", resp.Size)
	res.Logf("已找到：%d 条结果", resp.Size)
	for _, rec := range resp.Results {
		row := make([]string, 0, len(rec))
		for _, v := range rec {
			row = append(row, toString(v))
		}
		res.Rows = append(res.Rows, row)
	}
	res.Logf("查询状态：完成")
	res.Log = append(res.Log, "-----------------------------")
	return res
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
