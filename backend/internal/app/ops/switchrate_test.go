package ops

import (
	"errors"
	"testing"
)

type fakeCmd struct {
	val string
	err error
}

func (c fakeCmd) Result() (string, error) { return c.val, c.err }

func TestParseCounter(t *testing.T) {
	cases := []struct {
		name string
		cmd  fakeCmd
		want int64
	}{
		{"正常数字", fakeCmd{val: "42"}, 42},
		{"空串", fakeCmd{val: ""}, 0},
		{"错误(redis.Nil 等)", fakeCmd{err: errors.New("nil")}, 0},
		{"非数字", fakeCmd{val: "x"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseCounter(c.cmd); got != c.want {
				t.Errorf("parseCounter=%d, want %d", got, c.want)
			}
		})
	}
}
