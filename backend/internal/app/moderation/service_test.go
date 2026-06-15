package moderation

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRepo 可配置的 Repository 假实现。
type fakeRepo struct {
	count    int
	countErr error
}

func (f *fakeRepo) InsertLog(context.Context, LogInput) error { return nil }
func (f *fakeRepo) CountViolationsSince(context.Context, int, time.Time) (int, error) {
	return f.count, f.countErr
}
func (f *fakeRepo) ListLogs(context.Context, LogFilter) (LogResult, error) {
	return LogResult{}, nil
}

// fakeBanner 记录是否被调用。
type fakeBanner struct {
	banned int // 被封禁的 userID，0=未调用
	err    error
}

func (f *fakeBanner) BanUser(_ context.Context, userID int) error {
	f.banned = userID
	return f.err
}

func TestMaybeAutoBan(t *testing.T) {
	cfg := Config{AutoBanEnabled: true, BanThreshold: 10, ViolationWindowHours: 720}
	cases := []struct {
		name     string
		cfg      Config
		count    int
		countErr error
		wantBan  bool
	}{
		{"达阈值→封禁", cfg, 10, nil, true},
		{"超阈值→封禁", cfg, 15, nil, true},
		{"未达阈值→不封", cfg, 9, nil, false},
		{"未启用→不封", Config{AutoBanEnabled: false, BanThreshold: 10}, 99, nil, false},
		{"计数出错→不封", cfg, 99, errors.New("db"), false},
		{"阈值为0→不封", Config{AutoBanEnabled: true, BanThreshold: 0}, 99, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{count: c.count, countErr: c.countErr}
			banner := &fakeBanner{}
			svc := NewService(nil, nil)
			svc.SetRepository(repo)
			svc.SetUserBanner(banner)
			svc.maybeAutoBan(context.Background(), 42, c.cfg)
			if c.wantBan && banner.banned != 42 {
				t.Errorf("应封禁 user 42，实际 banned=%d", banner.banned)
			}
			if !c.wantBan && banner.banned != 0 {
				t.Errorf("不应封禁，实际 banned=%d", banner.banned)
			}
		})
	}
}

// fakeModerator 可配置的 APIModerator 假实现。
type fakeModerator struct {
	category string
	score    float64
	flagged  bool
	err      error
	called   bool
}

func (f *fakeModerator) Moderate(_ context.Context, _ string, _ Config) (string, float64, bool, error) {
	f.called = true
	return f.category, f.score, f.flagged, f.err
}

func userBody(text string) []byte {
	return []byte(`{"messages":[{"role":"user","content":"` + text + `"}]}`)
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name        string
		cfg         Config
		fake        *fakeModerator
		body        []byte
		wantBlocked bool
		wantSource  Source
		wantReject  bool
		wantAPICall bool
	}{
		{
			name:        "未启用 → 放行",
			cfg:         Config{Enabled: false, Mode: ModePreBlock},
			body:        userBody("anything"),
			wantBlocked: false,
		},
		{
			name:        "mode=off → 放行",
			cfg:         Config{Enabled: true, Mode: ModeOff},
			body:        userBody("anything"),
			wantBlocked: false,
		},
		{
			name:        "空文本 → 放行且不调 API",
			cfg:         Config{Enabled: true, Mode: ModePreBlock, KeywordMode: APIOnly},
			fake:        &fakeModerator{flagged: true},
			body:        []byte(`{"messages":[{"role":"assistant","content":"hi"}]}`),
			wantBlocked: false,
			wantAPICall: false,
		},
		{
			name:        "keyword_only 命中 → 拦截，不调 API",
			cfg:         Config{Enabled: true, Mode: ModePreBlock, KeywordMode: KeywordOnly, BlockedKeywords: []string{"BadWord"}},
			fake:        &fakeModerator{flagged: true},
			body:        userBody("this has a badword inside"),
			wantBlocked: true,
			wantSource:  SourceKeyword,
			wantReject:  true,
			wantAPICall: false,
		},
		{
			name:        "keyword_and_api：词库放过，API 命中 → 拦截",
			cfg:         Config{Enabled: true, Mode: ModePreBlock, KeywordMode: KeywordAndAPI, BlockedKeywords: []string{"nope"}},
			fake:        &fakeModerator{category: "violence", score: 0.99, flagged: true},
			body:        userBody("clean-ish text"),
			wantBlocked: true,
			wantSource:  SourceAPI,
			wantReject:  true,
			wantAPICall: true,
		},
		{
			name:        "API 出错 → 降级放行",
			cfg:         Config{Enabled: true, Mode: ModePreBlock, KeywordMode: APIOnly},
			fake:        &fakeModerator{err: errors.New("boom")},
			body:        userBody("text"),
			wantBlocked: false,
			wantAPICall: true,
		},
		{
			name:        "observe 命中 → Blocked 但不拒绝",
			cfg:         Config{Enabled: true, Mode: ModeObserve, KeywordMode: KeywordOnly, BlockedKeywords: []string{"bad"}},
			body:        userBody("a bad thing"),
			wantBlocked: true,
			wantSource:  SourceKeyword,
			wantReject:  false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var api APIModerator
			if c.fake != nil {
				api = c.fake
			}
			svc := NewService(api, nil)
			dec := svc.Check(context.Background(), c.cfg, CheckInput{Body: c.body})

			if dec.Blocked != c.wantBlocked {
				t.Errorf("Blocked = %v, want %v", dec.Blocked, c.wantBlocked)
			}
			if c.wantSource != "" && dec.Source != c.wantSource {
				t.Errorf("Source = %q, want %q", dec.Source, c.wantSource)
			}
			if dec.ShouldReject() != c.wantReject {
				t.Errorf("ShouldReject = %v, want %v", dec.ShouldReject(), c.wantReject)
			}
			if c.fake != nil && c.fake.called != c.wantAPICall {
				t.Errorf("API called = %v, want %v", c.fake.called, c.wantAPICall)
			}
		})
	}
}

func TestCheckDecisionDefaults(t *testing.T) {
	svc := NewService(nil, nil)
	dec := svc.Check(context.Background(), Config{Enabled: true, Mode: ModePreBlock, KeywordMode: KeywordOnly, BlockedKeywords: []string{"x"}}, CheckInput{Body: userBody("has x here")})
	if dec.StatusCode != defaultBlockStatus {
		t.Errorf("默认状态码 = %d, want %d", dec.StatusCode, defaultBlockStatus)
	}
	if dec.Message != defaultBlockMessage {
		t.Errorf("默认消息 = %q, want %q", dec.Message, defaultBlockMessage)
	}
}
