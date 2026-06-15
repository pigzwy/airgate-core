package compliance

import (
	"context"
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	settings "github.com/DouDOU-start/airgate-core/internal/app/settings"
)

//go:embed legal/admin-compliance.zh.md
var documentZh string

//go:embed legal/admin-compliance.en.md
var documentEn string

// Service 管理员合规确认门用例编排。
type Service struct {
	settings SettingsPort
}

// NewService 构造合规服务。settings 通常传 *app/settings.Service。
func NewService(s SettingsPort) *Service {
	return &Service{settings: s}
}

// Status 读取某管理员的合规确认状态。
//
// 任意一步读设置失败都返回 error，由调用方决定降级策略（中间件 fail-open）。
func (s *Service) Status(ctx context.Context, adminUserID int, language string) (Status, error) {
	lang := normalizeLang(language)
	st := Status{
		Version:    Version,
		Language:   lang,
		Phrase:     phraseFor(lang),
		DocumentZh: documentZh,
		DocumentEn: documentEn,
	}

	items, err := s.settings.List(ctx, settingGroup)
	if err != nil {
		return st, err
	}

	ackKey := ackKeyPrefix + strconv.Itoa(adminUserID)
	for _, it := range items {
		switch it.Key {
		case keyGateEnabled:
			st.Enabled = it.Value == "true"
		case ackKey:
			if it.Value != "" {
				var ack Acknowledgement
				if json.Unmarshal([]byte(it.Value), &ack) == nil {
					st.Ack = &ack
				}
			}
		}
	}

	st.Acknowledged = st.Ack != nil && st.Ack.Version == Version
	st.Required = st.Enabled && !st.Acknowledged
	return st, nil
}

// Gate 供中间件调用的轻量判定：是否应拦截 + 当前版本号。
// 读设置失败时返回 error，中间件据此 fail-open（绝不因运维故障锁死管理端）。
func (s *Service) Gate(ctx context.Context, adminUserID int) (blocked bool, version string, err error) {
	st, err := s.Status(ctx, adminUserID, "")
	if err != nil {
		return false, Version, err
	}
	return st.Required, st.Version, nil
}

// Accept 提交确认。短语须与当前语言要求逐字匹配，否则 ErrInvalidPhrase。
func (s *Service) Accept(ctx context.Context, in AcceptInput) (Status, error) {
	lang := normalizeLang(in.Language)
	if strings.TrimSpace(in.Phrase) != phraseFor(lang) {
		return Status{}, ErrInvalidPhrase
	}

	ack := Acknowledgement{
		Version:     Version,
		AdminUserID: in.AdminUserID,
		IPAddress:   in.IPAddress,
		UserAgent:   in.UserAgent,
		Language:    lang,
		AcceptedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(ack)
	if err != nil {
		return Status{}, err
	}

	if err := s.settings.Update(ctx, []settings.ItemInput{{
		Key:   ackKeyPrefix + strconv.Itoa(in.AdminUserID),
		Value: string(raw),
		Group: settingGroup,
	}}); err != nil {
		return Status{}, err
	}

	return s.Status(ctx, in.AdminUserID, lang)
}

// SetEnabled 开启/关闭合规门（管理员设置项；可由设置页或独立端点调用）。
func (s *Service) SetEnabled(ctx context.Context, enabled bool) error {
	val := ""
	if enabled {
		val = "true"
	}
	return s.settings.Update(ctx, []settings.ItemInput{{
		Key:   keyGateEnabled,
		Value: val,
		Group: settingGroup,
	}})
}

// normalizeLang 归一化语言，默认 zh。
func normalizeLang(language string) string {
	if strings.HasPrefix(strings.ToLower(language), "en") {
		return "en"
	}
	return "zh"
}

// phraseFor 返回指定语言的确认短语。
func phraseFor(lang string) string {
	if lang == "en" {
		return phraseEn
	}
	return phraseZh
}
