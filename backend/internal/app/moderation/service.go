package moderation

import (
	"context"
	"log/slog"
	"time"
)

const (
	defaultBlockStatus  = 403
	defaultBlockMessage = "内容命中风险规则，请调整输入后重试"
)

// Service 内容审核域用例编排。
//
// Check 是纯检测：抽文本 → 脱敏 → 关键词 → API → 判定，返回 Decision。
// 不碰 gin/http，不直接落库——封禁计数与日志落库由上层（forwarder 挂载点 + store，
// 第二批）基于 Decision 处理；这样核心检测逻辑可独立单测。
type Service struct {
	api    APIModerator
	repo   Repository     // 命中落库（nil 时不落，best-effort）
	cfg    ConfigProvider // 运行期配置来源（nil 时视为未配置=放行）
	banner UserBanner     // 自动封禁（nil 时不封）
	logger *slog.Logger
}

// NewService 构造审核服务。api 为 nil 时仅支持关键词模式（API 路径降级为放行）。
func NewService(api APIModerator, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{api: api, logger: logger}
}

// SetRepository 注入审核日志仓储。装配期调用。
func (s *Service) SetRepository(repo Repository) { s.repo = repo }

// SetConfigProvider 注入运行期配置来源。装配期调用。
func (s *Service) SetConfigProvider(cfg ConfigProvider) { s.cfg = cfg }

// SetUserBanner 注入自动封禁实现。装配期调用。
func (s *Service) SetUserBanner(b UserBanner) { s.banner = b }

// defaultBanWindow 违规计数窗口缺省值（30 天）。
const defaultBanWindow = 720 * time.Hour

// logRetention 异步落库的超时，独立于已结束的请求 context。
const logInsertTimeout = 3 * time.Second

// Evaluate 完整审核：加载配置 → 检测 → 命中异步落库，返回判定。
//
// 供 forwarder 在转发前调用。配置加载失败或未注入 → 放行（绝不因审核链路拖垮转发）。
// 命中落库为 fire-and-forget（goroutine + 独立 context），不给请求路径加 DB 延迟。
func (s *Service) Evaluate(ctx context.Context, in CheckInput) Decision {
	var cfg Config
	if s.cfg != nil {
		loaded, err := s.cfg.Load(ctx)
		if err != nil {
			s.logger.Warn("moderation_config_load_failed", "request_id", in.RequestID, "error", err)
			return Decision{} // 放行
		}
		cfg = loaded
	}

	dec := s.Check(ctx, cfg, in)

	if dec.Blocked && s.repo != nil {
		logIn := LogInput{
			RequestID: in.RequestID,
			UserID:    in.UserID,
			Platform:  in.Platform,
			Endpoint:  in.Endpoint,
			Mode:      dec.Mode,
			Flagged:   true,
			Source:    dec.Source,
			Category:  dec.Category,
			Score:     dec.Score,
			Excerpt:   dec.Excerpt,
			CreatedAt: time.Now(),
		}
		userID := in.UserID
		go func() {
			bctx, cancel := context.WithTimeout(context.Background(), logInsertTimeout)
			defer cancel()
			if err := s.repo.InsertLog(bctx, logIn); err != nil {
				s.logger.Warn("moderation_log_insert_failed", "request_id", logIn.RequestID, "error", err)
				return
			}
			// 日志已落库，count 包含本次命中——据此判断是否自动封禁
			s.maybeAutoBan(bctx, userID, cfg)
		}()
	}

	return dec
}

// maybeAutoBan 在窗口内命中次数达阈值时禁用用户。best-effort，仅记日志不返回错误。
func (s *Service) maybeAutoBan(ctx context.Context, userID int, cfg Config) {
	if !cfg.AutoBanEnabled || s.banner == nil || cfg.BanThreshold <= 0 || userID <= 0 {
		return
	}
	window := time.Duration(cfg.ViolationWindowHours) * time.Hour
	if window <= 0 {
		window = defaultBanWindow
	}
	count, err := s.repo.CountViolationsSince(ctx, userID, time.Now().Add(-window))
	if err != nil {
		s.logger.Warn("moderation_violation_count_failed", "user_id", userID, "error", err)
		return
	}
	if count < cfg.BanThreshold {
		return
	}
	if err := s.banner.BanUser(ctx, userID); err != nil {
		s.logger.Warn("moderation_auto_ban_failed", "user_id", userID, "error", err)
		return
	}
	s.logger.Info("moderation_user_auto_banned",
		"user_id", userID, "count", count, "threshold", cfg.BanThreshold)
}

// Check 对一次请求做内容审核，返回判定。
//
// 失败降级原则：审核 API 出错只记日志、按放行处理，绝不因审核链路抖动拖垮业务转发。
// 是否真正拒绝由调用方按 Decision.ShouldReject() 决定（仅 pre_block 命中才拒）。
func (s *Service) Check(ctx context.Context, cfg Config, input CheckInput) Decision {
	dec := Decision{
		Mode:       cfg.Mode,
		StatusCode: orInt(cfg.BlockStatus, defaultBlockStatus),
		Message:    orDefault(cfg.BlockMessage, defaultBlockMessage),
	}

	if !cfg.Enabled || cfg.Mode == ModeOff {
		return dec
	}

	text := ExtractUserText(input.Body)
	if text == "" {
		return dec // 无可审文本，放行
	}
	redacted := Redact(text)
	dec.Excerpt = truncate(redacted, maxExcerptLen)

	km := cfg.KeywordMode
	if km == "" {
		km = KeywordAndAPI
	}

	// 1) 本地关键词
	if km == KeywordOnly || km == KeywordAndAPI {
		if hit := matchKeyword(text, cfg.BlockedKeywords); hit != "" {
			dec.Blocked = true
			dec.Source = SourceKeyword
			dec.Category = "keyword"
			dec.Score = 1
			return dec
		}
	}

	// 2) OpenAI Moderation API
	if km == APIOnly || km == KeywordAndAPI {
		if s.api == nil {
			return dec // 无 API 实现，降级放行
		}
		// 送脱敏后文本，避免把机密推给第三方
		cat, score, flagged, err := s.api.Moderate(ctx, redacted, cfg)
		if err != nil {
			s.logger.Warn("moderation_api_failed",
				"request_id", input.RequestID, "user_id", input.UserID, "error", err)
			return dec // 失败降级放行
		}
		if flagged {
			dec.Blocked = true
			dec.Source = SourceAPI
			dec.Category = cat
			dec.Score = score
		}
	}

	return dec
}

// ShouldReject 是否应拒绝该请求：仅 pre_block 模式下命中才拒。
// observe 模式即便命中也放行（只记录/通知）。
func (d Decision) ShouldReject() bool {
	return d.Blocked && d.Mode == ModePreBlock
}
