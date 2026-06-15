package compliance

import "errors"

// ErrInvalidPhrase 确认短语与当前语言要求的短语不匹配。
var ErrInvalidPhrase = errors.New("合规确认短语不正确")
