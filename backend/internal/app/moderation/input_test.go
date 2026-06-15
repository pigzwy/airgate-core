package moderation

import "testing"

func TestExtractUserText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"字符串 content", `{"messages":[{"role":"user","content":"hello world"}]}`, "hello world"},
		{"取最后一条 user", `{"messages":[{"role":"user","content":"first"},{"role":"assistant","content":"hi"},{"role":"user","content":"second"}]}`, "second"},
		{"块数组 content", `{"messages":[{"role":"user","content":[{"type":"text","text":"a"},{"type":"image"},{"type":"text","text":"b"}]}]}`, "a\nb"},
		{"无 user 消息", `{"messages":[{"role":"assistant","content":"hi"}]}`, ""},
		{"非法 JSON", `not json`, ""},
		{"空 body", ``, ""},
		{"无 messages", `{"model":"gpt"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExtractUserText([]byte(c.body)); got != c.want {
				t.Errorf("ExtractUserText = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRedact(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"OpenAI key", "my key sk-abcdefghijklmnop1234 end", "my key [已脱敏] end"},
		{"Bearer", "auth Bearer abcdefghijklmnop.token end", "auth [已脱敏] end"},
		{"URL", "see https://example.com/path?x=1 now", "see [已脱敏] now"},
		{"邮箱", "mail a.b@example.com here", "mail [已脱敏] here"},
		{"无敏感", "just plain text", "just plain text"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Redact(c.in); got != c.want {
				t.Errorf("Redact(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestExcerptTruncates(t *testing.T) {
	long := make([]rune, maxExcerptLen+50)
	for i := range long {
		long[i] = 'a'
	}
	got := Excerpt(string(long))
	if len([]rune(got)) != maxExcerptLen {
		t.Errorf("Excerpt 长度 = %d, want %d", len([]rune(got)), maxExcerptLen)
	}
}
