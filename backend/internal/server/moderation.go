package server

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/DouDOU-start/airgate-core/ent"
	appmoderation "github.com/DouDOU-start/airgate-core/internal/app/moderation"
	appsettings "github.com/DouDOU-start/airgate-core/internal/app/settings"
	"github.com/DouDOU-start/airgate-core/internal/auth"
	"github.com/DouDOU-start/airgate-core/internal/infra/store"
)

// moderationConfigKey 审核配置在 settings 表的 key（group="moderation"）。
const moderationConfigKey = "moderation_config"
const moderationConfigGroup = "moderation"

// newModerationService 构造内容审核服务并接好依赖：
//   - OpenAI Moderation API 客户端
//   - ent 仓储（命中落库）
//   - settings 配置来源（解密 API key）
//
// 未配置 settings 时 Load 返回禁用配置 → 审核整体放行（默认关闭，安全）。
func newModerationService(db *ent.Client, secret string) *appmoderation.Service {
	svc := appmoderation.NewService(appmoderation.NewOpenAIModerator(), slog.Default())
	svc.SetRepository(store.NewModerationStore(db))
	svc.SetConfigProvider(&moderationConfigProvider{
		settings: store.NewSettingsStore(db),
		secret:   secret,
	})
	return svc
}

// moderationConfigProvider 从 settings(group=moderation) 读取并解密审核配置。
type moderationConfigProvider struct {
	settings appsettings.Repository
	secret   string
}

var _ appmoderation.ConfigProvider = (*moderationConfigProvider)(nil)

// moderationConfigJSON settings 中存储的审核配置 JSON 形态。
type moderationConfigJSON struct {
	Enabled              bool               `json:"enabled"`
	Mode                 string             `json:"mode"`
	KeywordMode          string             `json:"keyword_mode"`
	BlockedKeywords      []string           `json:"blocked_keywords"`
	APIBaseURL           string             `json:"api_base_url"`
	APIModel             string             `json:"api_model"`
	APIKeys              []string           `json:"api_keys"` // 加密存储
	TimeoutMs            int                `json:"timeout_ms"`
	RetryCount           int                `json:"retry_count"`
	Thresholds           map[string]float64 `json:"thresholds"`
	BlockStatus          int                `json:"block_status"`
	BlockMessage         string             `json:"block_message"`
	AutoBanEnabled       bool               `json:"auto_ban_enabled"`
	BanThreshold         int                `json:"ban_threshold"`
	ViolationWindowHours int                `json:"violation_window_hours"`
}

func (p *moderationConfigProvider) Load(ctx context.Context) (appmoderation.Config, error) {
	items, err := p.settings.List(ctx, moderationConfigGroup)
	if err != nil {
		return appmoderation.Config{}, err
	}
	var raw string
	for _, it := range items {
		if it.Key == moderationConfigKey {
			raw = it.Value
		}
	}
	if raw == "" {
		return appmoderation.Config{}, nil // 未配置 = 禁用
	}

	var j moderationConfigJSON
	if err := json.Unmarshal([]byte(raw), &j); err != nil {
		return appmoderation.Config{}, err
	}

	// 解密 API key（明文绝不落库；解密失败的单个 key 跳过，不阻断其余）
	keys := make([]string, 0, len(j.APIKeys))
	for _, enc := range j.APIKeys {
		if enc == "" {
			continue
		}
		dec, derr := auth.DecryptAPIKey(enc, p.secret)
		if derr != nil {
			slog.Warn("moderation_api_key_decrypt_failed", "error", derr)
			continue
		}
		keys = append(keys, dec)
	}

	return appmoderation.Config{
		Enabled:              j.Enabled,
		Mode:                 appmoderation.Mode(j.Mode),
		KeywordMode:          appmoderation.KeywordMode(j.KeywordMode),
		BlockedKeywords:      j.BlockedKeywords,
		APIBaseURL:           j.APIBaseURL,
		APIModel:             j.APIModel,
		APIKeys:              keys,
		TimeoutMs:            j.TimeoutMs,
		RetryCount:           j.RetryCount,
		Thresholds:           j.Thresholds,
		BlockStatus:          j.BlockStatus,
		BlockMessage:         j.BlockMessage,
		AutoBanEnabled:       j.AutoBanEnabled,
		BanThreshold:         j.BanThreshold,
		ViolationWindowHours: j.ViolationWindowHours,
	}, nil
}
