package ops

import "errors"

var (
	// ErrRequestLogNotFound 请求日志不存在。
	ErrRequestLogNotFound = errors.New("请求日志不存在")
	// ErrInvalidTimeRange 时间范围参数非法。
	ErrInvalidTimeRange = errors.New("时间范围无效")
	// ErrAlertRuleNotFound 告警规则不存在。
	ErrAlertRuleNotFound = errors.New("告警规则不存在")
	// ErrInvalidAlertRule 告警规则参数非法。
	ErrInvalidAlertRule = errors.New("告警规则参数无效")
)
