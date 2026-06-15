package plugin

import (
	"context"

	"github.com/gin-gonic/gin"

	appmoderation "github.com/DouDOU-start/airgate-core/internal/app/moderation"
	"github.com/DouDOU-start/airgate-core/internal/server/middleware"
)

// moderationGate 内容审核网关，由 app/moderation.Service 满足。
// 在 plugin 包定义最小接口，避免转发热路径强耦合具体类型，也便于不注入时 nil 跳过。
type moderationGate interface {
	Evaluate(ctx context.Context, in appmoderation.CheckInput) appmoderation.Decision
}

// SetModeration 注入内容审核网关。装配期调用一次；未注入时审核被跳过（放行）。
func (f *Forwarder) SetModeration(g moderationGate) {
	f.moderation = g
}

// checkModeration 转发前内容审核预检（在 checkBalance 之后、选号之前）。
//
// 设计：
//   - 未注入网关 / 只读元信息路径（无内容可审）→ 直接放行。
//   - 仅 pre_block 命中才拒绝（observe 模式由 Evaluate 内部记录、此处放行）。
//   - 命中拒绝返回审核配置的状态码 + 消息（OpenAI 兼容错误体）。
//   - 失败降级（配置加载/API 出错）由 Evaluate 内部按放行处理，绝不拖垮转发。
func (f *Forwarder) checkModeration(c *gin.Context, state *forwardState) bool {
	if f.moderation == nil || isMetadataOnlyPath(state.requestPath) {
		return true
	}

	dec := f.moderation.Evaluate(c.Request.Context(), appmoderation.CheckInput{
		RequestID:   middleware.RequestIDFromGinContext(c),
		UserID:      state.keyInfo.UserID,
		Platform:    state.requestedPlatform,
		Endpoint:    state.requestPath,
		Body:        state.body,
		ContentType: c.GetHeader("Content-Type"),
	})

	if dec.ShouldReject() {
		openAIError(c, dec.StatusCode, "invalid_request_error", "content_policy_violation", dec.Message)
		return false
	}
	return true
}
