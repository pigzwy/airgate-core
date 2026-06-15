package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 默认 OpenAI Moderation 配置。
const (
	defaultAPIBaseURL = "https://api.openai.com"
	defaultAPIModel   = "omni-moderation-latest"
	defaultTimeoutMs  = 3000
)

// OpenAIModerator 调用 OpenAI Moderation API 的 APIModerator 实现。
//
// 多 key 轮询：按 key 顺序尝试，遇到鉴权/限流类错误换下一个；全部失败返回错误。
// 失败由上层决定降级策略（observe 记录、pre_block 可选 fail-open，避免审核 API
// 抖动直接拖垮转发——见 service）。
type OpenAIModerator struct {
	httpClient *http.Client
}

// NewOpenAIModerator 创建审核 API 客户端。
func NewOpenAIModerator() *OpenAIModerator {
	return &OpenAIModerator{httpClient: &http.Client{}}
}

var _ APIModerator = (*OpenAIModerator)(nil)

type moderationAPIResponse struct {
	Results []struct {
		Flagged        bool               `json:"flagged"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
}

// Moderate 调用 API，返回超过阈值的最高分分类。
//
// 判定：取 category_scores 中相对各自阈值"超出最多"的分类作为命中分类；
// 任一分类 score ≥ 阈值即 flagged=true。无 key 返回 ErrNoAPIKey。
func (m *OpenAIModerator) Moderate(ctx context.Context, text string, cfg Config) (string, float64, bool, error) {
	keys := nonEmpty(cfg.APIKeys)
	if len(keys) == 0 {
		return "", 0, false, ErrNoAPIKey
	}

	baseURL := strings.TrimRight(orDefault(cfg.APIBaseURL, defaultAPIBaseURL), "/")
	model := orDefault(cfg.APIModel, defaultAPIModel)
	timeout := time.Duration(orInt(cfg.TimeoutMs, defaultTimeoutMs)) * time.Millisecond
	thresholds := cfg.Thresholds
	if len(thresholds) == 0 {
		thresholds = DefaultThresholds()
	}

	reqBody, _ := json.Marshal(map[string]any{"model": model, "input": text})

	var lastErr error
	for _, key := range keys {
		scores, err := m.callOnce(ctx, baseURL, key, reqBody, timeout)
		if err != nil {
			lastErr = err
			continue // 换下一个 key
		}
		cat, score, flagged := evaluateScores(scores, thresholds)
		return cat, score, flagged, nil
	}
	return "", 0, false, fmt.Errorf("moderation api 全部 key 失败: %w", lastErr)
}

func (m *OpenAIModerator) callOnce(ctx context.Context, baseURL, key string, reqBody []byte, timeout time.Duration) (map[string]float64, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, baseURL+"/v1/moderations", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moderation api 状态码 %d", resp.StatusCode)
	}
	var parsed moderationAPIResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Results) == 0 {
		return nil, fmt.Errorf("moderation api 空结果")
	}
	return parsed.Results[0].CategoryScores, nil
}

// evaluateScores 在分类分值里找"相对阈值超出最多"的分类。任一超阈即 flagged。
func evaluateScores(scores, thresholds map[string]float64) (category string, score float64, flagged bool) {
	bestMargin := 0.0
	for cat, sc := range scores {
		th, ok := thresholds[cat]
		if !ok {
			continue // 未配阈值的分类不参与判定
		}
		if sc >= th {
			flagged = true
			margin := sc - th
			if category == "" || margin > bestMargin {
				bestMargin = margin
				category = cat
				score = sc
			}
		}
	}
	return category, score, flagged
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func orInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
