package moderation

import "strings"

// matchKeyword 在 text 中查找命中的敏感词（子串、不区分大小写）。
// 返回命中的词；未命中返回空串。空词库直接放行。
func matchKeyword(text string, keywords []string) string {
	if text == "" || len(keywords) == 0 {
		return ""
	}
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(kw)) {
			return kw
		}
	}
	return ""
}
