package moderation

import "errors"

var (
	// ErrBlocked 内容命中审核规则（pre_block 模式下用于上层映射拒绝响应）。
	ErrBlocked = errors.New("内容命中审核规则")
	// ErrNoAPIKey 配置为需调 API 但未提供任何 key。
	ErrNoAPIKey = errors.New("未配置 OpenAI Moderation API key")
)
