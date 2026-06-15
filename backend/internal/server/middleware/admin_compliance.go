package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"

	"github.com/DouDOU-start/airgate-core/internal/server/response"
)

// compliancePathPrefix 合规门自身端点前缀，须放行（否则未确认时无法获取文档/提交确认，死锁）。
const compliancePathPrefix = "/api/v1/admin/compliance"

// ComplianceGate 合规门判定能力，由 app/compliance.Service 满足。
// 返回 (是否拦截, 当前版本号, 错误)；读设置出错时由本中间件 fail-open。
type ComplianceGate interface {
	Gate(ctx context.Context, adminUserID int) (blocked bool, version string, err error)
}

// AdminComplianceGate 管理员合规确认门中间件。
//
// 必须挂在 JWTAuth + AdminOnly 之后（依赖 ctx 中的 user_id / admin 角色）。
// 行为：
//   - 放行合规门自身端点（获取状态 / 提交确认）；
//   - 管理员 API Key（user_id==0，机器凭证）不拦——脚本化调用不应被交互式确认阻断；
//   - 未确认当前版本合规声明时返回 423 Locked，前端据此弹确认框；
//   - 读设置失败时放行（fail-open），绝不因运维故障锁死整个管理端。
//
// 合规门默认关闭（compliance_gate_enabled 未开启时 Gate 恒返回 blocked=false），
// 需管理员显式开启后才生效——避免在前端支持 423 之前破坏运行中的后台。
func AdminComplianceGate(gate ComplianceGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 放行合规门自身端点
		if strings.HasPrefix(c.Request.URL.Path, compliancePathPrefix) {
			c.Next()
			return
		}

		// 取 user_id；缺失或为 0（管理员 API Key）不拦
		v, exists := c.Get(CtxKeyUserID)
		uid, ok := v.(int)
		if !exists || !ok || uid == 0 {
			c.Next()
			return
		}

		blocked, version, err := gate.Gate(c.Request.Context(), uid)
		if err != nil {
			// 读设置失败：放行并记日志，不锁死管理端
			sdk.LoggerFromContext(c.Request.Context()).Warn("admin_compliance_gate_degraded",
				sdk.LogFieldError, err,
				sdk.LogFieldRequestID, RequestIDFromGinContext(c))
			c.Next()
			return
		}
		if !blocked {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusLocked, response.R{
			Code:    http.StatusLocked,
			Message: "需要管理员合规确认",
			Data: gin.H{
				"code":    "ADMIN_COMPLIANCE_ACK_REQUIRED",
				"version": version,
			},
		})
	}
}
