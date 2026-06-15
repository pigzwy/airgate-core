package ops

import (
	"context"
	"testing"
	"time"
)

func TestDurationDays(t *testing.T) {
	def := 7 * 24 * time.Hour
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", def},
		{"0", def},
		{"-3", def},
		{"abc", def},
		{"1", 24 * time.Hour},
		{"30", 30 * 24 * time.Hour},
	}
	for _, c := range cases {
		if got := durationDays(c.in, def); got != c.want {
			t.Errorf("durationDays(%q)=%v, want %v", c.in, got, c.want)
		}
	}
}

// staticConfig 是 ConfigProvider 的固定返回测试替身。
type staticConfig map[string]string

func (s staticConfig) OpsSettings(ctx context.Context) map[string]string { return s }

func TestRetentionDurations_DefaultsAndOverride(t *testing.T) {
	// 无 provider → 默认
	svc := NewService(newFakeRepo(), nil)
	l, st, sy := svc.retentionDurations(context.Background())
	if l != defaultLogRetention || st != defaultStatRetention || sy != defaultSysLogRetention {
		t.Errorf("默认保留期不对: %v %v %v", l, st, sy)
	}

	// 有 provider → 覆盖
	svc.SetConfigProvider(staticConfig{
		"ops_retention_log_days":    "3",
		"ops_retention_stat_days":   "10",
		"ops_retention_syslog_days": "5",
	})
	l, st, sy = svc.retentionDurations(context.Background())
	if l != 3*24*time.Hour || st != 10*24*time.Hour || sy != 5*24*time.Hour {
		t.Errorf("覆盖保留期不对: %v %v %v", l, st, sy)
	}
}

func TestDailyReportConfig(t *testing.T) {
	svc := NewService(newFakeRepo(), nil)
	// 默认：未启用、hour=9
	cfg := svc.dailyReportConfig(context.Background())
	if cfg.enabled || cfg.hour != 9 {
		t.Errorf("默认日报配置不对: %+v", cfg)
	}
	svc.SetConfigProvider(staticConfig{
		"ops_report_enabled": "true",
		"ops_report_email":   "a@b.com",
		"ops_report_hour":    "23",
	})
	cfg = svc.dailyReportConfig(context.Background())
	if !cfg.enabled || cfg.email != "a@b.com" || cfg.hour != 23 {
		t.Errorf("日报配置覆盖不对: %+v", cfg)
	}
	// 非法 hour 回退默认 9
	svc.SetConfigProvider(staticConfig{"ops_report_hour": "99"})
	if svc.dailyReportConfig(context.Background()).hour != 9 {
		t.Error("非法 hour 应回退 9")
	}
}
