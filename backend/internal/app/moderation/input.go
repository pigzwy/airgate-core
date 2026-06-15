package moderation

import (
	"encoding/json"
	"regexp"
	"strings"
)

// maxExtractLen 待审文本最大长度（超出截断），避免把超大 body 推给审核 API。
const maxExtractLen = 12000

// maxExcerptLen 落日志/邮件用的摘录长度。
const maxExcerptLen = 240

// ExtractUserText 从请求体抽取"最后一条 user 消息"的纯文本。
//
// 支持两类主流协议（MVP）：
//   - Anthropic Messages / OpenAI Chat：{"messages":[{"role":"user","content":...}]}
//     content 可为字符串，或 [{"type":"text","text":"..."}] 块数组。
//
// 解析失败或无 user 消息时返回空串（上层据此放行）。
func ExtractUserText(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	// 取最后一条 role=user 的消息
	for i := len(payload.Messages) - 1; i >= 0; i-- {
		m := payload.Messages[i]
		if m.Role != "user" {
			continue
		}
		text := contentToText(m.Content)
		return truncate(strings.TrimSpace(text), maxExtractLen)
	}
	return ""
}

// contentToText 把 message.content 归一化为文本：字符串原样；块数组拼接其中 text 块。
func contentToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// 情况一：字符串
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// 情况二：块数组
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(b.Text)
			}
		}
		return sb.String()
	}
	return ""
}

// 脱敏正则：把机密信息替换为占位符，避免把 key/token 等推给第三方审核 API 或落日志。
var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_\-]{16,}`),                       // OpenAI 风格 key
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-]{16,}`),                // Bearer token
	regexp.MustCompile(`\beyJ[A-Za-z0-9._\-]{10,}`),                          // JWT
	regexp.MustCompile(`https?://[^\s]+`),                                    // URL
	regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`), // 邮箱
}

const redactPlaceholder = "[已脱敏]"

// Redact 脱敏文本中的机密信息。
func Redact(text string) string {
	for _, re := range redactPatterns {
		text = re.ReplaceAllString(text, redactPlaceholder)
	}
	return text
}

// Excerpt 返回脱敏并截断后的摘录（落日志/邮件用）。
func Excerpt(text string) string {
	return truncate(Redact(text), maxExcerptLen)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// 按 rune 边界截断，避免切碎多字节字符
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
