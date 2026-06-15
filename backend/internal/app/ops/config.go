package ops

import (
	"context"
	"strconv"
	"time"
)

// ===== 运维设置（M15）：从 settings(group=ops) 读取可调配置 =====

// ConfigProvider 提供运维可调配置（实现读 settings group=ops）。
type ConfigProvider interface {
	OpsSettings(ctx context.Context) map[string]string
}

// SetConfigProvider 注入配置来源。装配期调用一次。
func (s *Service) SetConfigProvider(p ConfigProvider) {
	s.config = p
}

// opsSettings 读取配置 map；未注入时返回空（走默认值）。
func (s *Service) opsSettings(ctx context.Context) map[string]string {
	if s.config == nil {
		return map[string]string{}
	}
	m := s.config.OpsSettings(ctx)
	if m == nil {
		return map[string]string{}
	}
	return m
}

// retentionDurations 解析三类保留期（天），缺省走 default 常量。
func (s *Service) retentionDurations(ctx context.Context) (logRet, statRet, sysRet time.Duration) {
	m := s.opsSettings(ctx)
	logRet = durationDays(m["ops_retention_log_days"], defaultLogRetention)
	statRet = durationDays(m["ops_retention_stat_days"], defaultStatRetention)
	sysRet = durationDays(m["ops_retention_syslog_days"], defaultSysLogRetention)
	return
}

// durationDays 把"天"字符串转 Duration；空或非法走 def。
func durationDays(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := strconv.Atoi(s)
	if err != nil || d <= 0 {
		return def
	}
	return time.Duration(d) * 24 * time.Hour
}

// dailyReportConfig 邮件日报配置。
type dailyReportConfig struct {
	enabled bool
	email   string
	hour    int // 0-23，本地时区
}

func (s *Service) dailyReportConfig(ctx context.Context) dailyReportConfig {
	m := s.opsSettings(ctx)
	cfg := dailyReportConfig{
		enabled: m["ops_report_enabled"] == "true",
		email:   m["ops_report_email"],
		hour:    9,
	}
	if h, err := strconv.Atoi(m["ops_report_hour"]); err == nil && h >= 0 && h <= 23 {
		cfg.hour = h
	}
	return cfg
}
