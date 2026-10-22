package api

import (
	"strconv"
	"strings"
)

// parseIDList 解析逗号分隔的正整数 ID 列表。
func parseIDList(raw string) []int {
	out := []int{}
	for _, s := range strings.Split(raw, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}
