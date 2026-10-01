package engines

import "fmt"

// SearchResult 统一返回：rows 供表格渲染，log 为日志行，extra 携带计数/积分/游标等
type SearchResult struct {
	Rows  [][]string        `json:"rows"`
	Log   []string          `json:"log"`
	Extra map[string]string `json:"extra"`
}

// NewResult 构造空结果
func NewResult() SearchResult {
	return SearchResult{Rows: [][]string{}, Log: []string{}, Extra: map[string]string{}}
}

// logf 追加一条日志
func (r *SearchResult) Logf(format string, a ...interface{}) SearchResult {
	r.Log = append(r.Log, fmt.Sprintf(format, a...))
	return *r
}
