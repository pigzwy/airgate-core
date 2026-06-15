package moderation

import (
	"context"
	"encoding/json"

	settings "github.com/DouDOU-start/airgate-core/internal/app/settings"
)

// settings 存储约定。
const (
	SettingGroup     = "moderation"
	SettingKeyConfig = "moderation_config"
)

// ConfigWire 是 settings 中存储的审核配置 JSON 形态（api_keys 为加密态）。
// 同时被 server 侧 ConfigProvider（读+解密）与 AdminService（读写）使用，单一来源。
type ConfigWire struct {
	Enabled              bool               `json:"enabled"`
	Mode                 string             `json:"mode"`
	KeywordMode          string             `json:"keyword_mode"`
	BlockedKeywords      []string           `json:"blocked_keywords"`
	APIBaseURL           string             `json:"api_base_url"`
	APIModel             string             `json:"api_model"`
	APIKeysEnc           []string           `json:"api_keys"` // 加密存储
	TimeoutMs            int                `json:"timeout_ms"`
	RetryCount           int                `json:"retry_count"`
	Thresholds           map[string]float64 `json:"thresholds"`
	BlockStatus          int                `json:"block_status"`
	BlockMessage         string             `json:"block_message"`
	AutoBanEnabled       bool               `json:"auto_ban_enabled"`
	BanThreshold         int                `json:"ban_threshold"`
	ViolationWindowHours int                `json:"violation_window_hours"`
}

// ParseConfigWire 解析 settings 原始 JSON；空串返回零值（=禁用）。
func ParseConfigWire(raw string) (ConfigWire, error) {
	var w ConfigWire
	if raw == "" {
		return w, nil
	}
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return ConfigWire{}, err
	}
	return w, nil
}

// SettingsRW 管理配置读写所需的设置能力（由 app/settings.Service 满足）。
type SettingsRW interface {
	List(ctx context.Context, group string) ([]settings.Setting, error)
	Update(ctx context.Context, items []settings.ItemInput) error
}

// Encryptor API key 加解密（由 bootstrap 用 auth.Encrypt/DecryptAPIKey + secret 适配）。
type Encryptor interface {
	Encrypt(plain string) (string, error)
	Decrypt(enc string) (string, error)
}

// ConfigView 配置读视图：api_keys 仅返回脱敏 hint，不回明文。
type ConfigView struct {
	Enabled              bool               `json:"enabled"`
	Mode                 string             `json:"mode"`
	KeywordMode          string             `json:"keyword_mode"`
	BlockedKeywords      []string           `json:"blocked_keywords"`
	APIBaseURL           string             `json:"api_base_url"`
	APIModel             string             `json:"api_model"`
	APIKeyHints          []string           `json:"api_key_hints"` // 脱敏：sk-…wxyz
	TimeoutMs            int                `json:"timeout_ms"`
	RetryCount           int                `json:"retry_count"`
	Thresholds           map[string]float64 `json:"thresholds"`
	BlockStatus          int                `json:"block_status"`
	BlockMessage         string             `json:"block_message"`
	AutoBanEnabled       bool               `json:"auto_ban_enabled"`
	BanThreshold         int                `json:"ban_threshold"`
	ViolationWindowHours int                `json:"violation_window_hours"`
}

// ConfigInput 配置写入：api_keys 为明文。空列表表示"保留原有 key 不变"。
type ConfigInput struct {
	Enabled              bool
	Mode                 string
	KeywordMode          string
	BlockedKeywords      []string
	APIBaseURL           string
	APIModel             string
	APIKeys              []string // 明文；空=保留原值
	TimeoutMs            int
	RetryCount           int
	Thresholds           map[string]float64
	BlockStatus          int
	BlockMessage         string
	AutoBanEnabled       bool
	BanThreshold         int
	ViolationWindowHours int
}

// AdminService 管理后台的审核配置读写与日志查询。
type AdminService struct {
	settings SettingsRW
	repo     Repository
	crypto   Encryptor
}

// NewAdminService 构造审核管理服务。
func NewAdminService(s SettingsRW, repo Repository, crypto Encryptor) *AdminService {
	return &AdminService{settings: s, repo: repo, crypto: crypto}
}

func (a *AdminService) loadWire(ctx context.Context) (ConfigWire, error) {
	items, err := a.settings.List(ctx, SettingGroup)
	if err != nil {
		return ConfigWire{}, err
	}
	var raw string
	for _, it := range items {
		if it.Key == SettingKeyConfig {
			raw = it.Value
		}
	}
	return ParseConfigWire(raw)
}

// GetConfig 读取配置，api_keys 脱敏为 hint。
func (a *AdminService) GetConfig(ctx context.Context) (ConfigView, error) {
	w, err := a.loadWire(ctx)
	if err != nil {
		return ConfigView{}, err
	}
	hints := make([]string, 0, len(w.APIKeysEnc))
	for _, enc := range w.APIKeysEnc {
		if enc == "" {
			continue
		}
		plain, derr := a.crypto.Decrypt(enc)
		if derr != nil {
			hints = append(hints, "sk-…?")
			continue
		}
		hints = append(hints, maskKey(plain))
	}
	return ConfigView{
		Enabled: w.Enabled, Mode: w.Mode, KeywordMode: w.KeywordMode,
		BlockedKeywords: w.BlockedKeywords, APIBaseURL: w.APIBaseURL, APIModel: w.APIModel,
		APIKeyHints: hints, TimeoutMs: w.TimeoutMs, RetryCount: w.RetryCount,
		Thresholds: w.Thresholds, BlockStatus: w.BlockStatus, BlockMessage: w.BlockMessage,
		AutoBanEnabled: w.AutoBanEnabled, BanThreshold: w.BanThreshold, ViolationWindowHours: w.ViolationWindowHours,
	}, nil
}

// SaveConfig 写入配置。api_keys 明文加密落库；input.APIKeys 为空时保留原有 key。
func (a *AdminService) SaveConfig(ctx context.Context, in ConfigInput) error {
	existing, err := a.loadWire(ctx)
	if err != nil {
		return err
	}

	encKeys := existing.APIKeysEnc // 默认保留
	if len(in.APIKeys) > 0 {
		encKeys = make([]string, 0, len(in.APIKeys))
		for _, plain := range in.APIKeys {
			if plain == "" {
				continue
			}
			enc, eerr := a.crypto.Encrypt(plain)
			if eerr != nil {
				return eerr
			}
			encKeys = append(encKeys, enc)
		}
	}

	w := ConfigWire{
		Enabled: in.Enabled, Mode: in.Mode, KeywordMode: in.KeywordMode,
		BlockedKeywords: in.BlockedKeywords, APIBaseURL: in.APIBaseURL, APIModel: in.APIModel,
		APIKeysEnc: encKeys, TimeoutMs: in.TimeoutMs, RetryCount: in.RetryCount,
		Thresholds: in.Thresholds, BlockStatus: in.BlockStatus, BlockMessage: in.BlockMessage,
		AutoBanEnabled: in.AutoBanEnabled, BanThreshold: in.BanThreshold, ViolationWindowHours: in.ViolationWindowHours,
	}
	raw, err := json.Marshal(w)
	if err != nil {
		return err
	}
	return a.settings.Update(ctx, []settings.ItemInput{{
		Key: SettingKeyConfig, Value: string(raw), Group: SettingGroup,
	}})
}

// ListLogs 审核日志分页（管理后台）。
func (a *AdminService) ListLogs(ctx context.Context, f LogFilter) (LogResult, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	return a.repo.ListLogs(ctx, f)
}

// maskKey 脱敏 API key：保留前缀与末 4 位。
func maskKey(plain string) string {
	if len(plain) <= 8 {
		return "sk-…"
	}
	return plain[:3] + "…" + plain[len(plain)-4:]
}
