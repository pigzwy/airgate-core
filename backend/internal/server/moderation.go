package server

import (
	"context"
	"log/slog"

	"github.com/DouDOU-start/airgate-core/ent"
	appmoderation "github.com/DouDOU-start/airgate-core/internal/app/moderation"
	appsettings "github.com/DouDOU-start/airgate-core/internal/app/settings"
	appuser "github.com/DouDOU-start/airgate-core/internal/app/user"
	"github.com/DouDOU-start/airgate-core/internal/auth"
	"github.com/DouDOU-start/airgate-core/internal/infra/store"
)

// newModerationService 构造内容审核检测服务并接好依赖：
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
	// 自动封禁：命中达阈值时把用户标记为禁用（app/user.Service 满足 UserBanner）。
	svc.SetUserBanner(appuser.NewService(store.NewUserStore(db)))
	return svc
}

// moderationConfigProvider 从 settings(group=moderation) 读取并解密审核配置。
// 复用 app/moderation.ConfigWire 作为存储 JSON 形态（与管理端写入单一来源）。
type moderationConfigProvider struct {
	settings appsettings.Repository
	secret   string
}

var _ appmoderation.ConfigProvider = (*moderationConfigProvider)(nil)

func (p *moderationConfigProvider) Load(ctx context.Context) (appmoderation.Config, error) {
	items, err := p.settings.List(ctx, appmoderation.SettingGroup)
	if err != nil {
		return appmoderation.Config{}, err
	}
	var raw string
	for _, it := range items {
		if it.Key == appmoderation.SettingKeyConfig {
			raw = it.Value
		}
	}
	w, err := appmoderation.ParseConfigWire(raw)
	if err != nil {
		return appmoderation.Config{}, err
	}

	// 解密 API key（明文绝不落库；单个解密失败跳过，不阻断其余）
	keys := make([]string, 0, len(w.APIKeysEnc))
	for _, enc := range w.APIKeysEnc {
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
		Enabled:              w.Enabled,
		Mode:                 appmoderation.Mode(w.Mode),
		KeywordMode:          appmoderation.KeywordMode(w.KeywordMode),
		BlockedKeywords:      w.BlockedKeywords,
		APIBaseURL:           w.APIBaseURL,
		APIModel:             w.APIModel,
		APIKeys:              keys,
		TimeoutMs:            w.TimeoutMs,
		RetryCount:           w.RetryCount,
		Thresholds:           w.Thresholds,
		BlockStatus:          w.BlockStatus,
		BlockMessage:         w.BlockMessage,
		AutoBanEnabled:       w.AutoBanEnabled,
		BanThreshold:         w.BanThreshold,
		ViolationWindowHours: w.ViolationWindowHours,
	}, nil
}
