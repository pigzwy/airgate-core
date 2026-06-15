package ops

import (
	"context"
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		" error ": slog.LevelError,
		"unknown": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q)=%v, want %v", in, got, want)
		}
	}
}

func TestLogSink_LevelControl(t *testing.T) {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelInfo)
	sink := NewLogSink(newFakeRepo(), slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: lv}), lv)

	if sink.Level() != "info" || sink.DefaultLevel() != "info" {
		t.Fatalf("初始级别不对: cur=%s def=%s", sink.Level(), sink.DefaultLevel())
	}
	// 抬高到 warn → info 不再 Enabled
	_ = sink.SetLevel("warn")
	if sink.Level() != "warn" {
		t.Errorf("SetLevel 后应为 warn，实际 %s", sink.Level())
	}
	if sink.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("warn 级别下 info 不应 Enabled")
	}
	if !sink.Enabled(context.Background(), slog.LevelError) {
		t.Error("warn 级别下 error 应 Enabled")
	}
	// 重置回默认
	sink.ResetLevel()
	if sink.Level() != "info" {
		t.Errorf("ResetLevel 后应回 info，实际 %s", sink.Level())
	}
}

func TestLogSink_HandleEnqueuesAndChainsInner(t *testing.T) {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelDebug)
	repo := newFakeRepo()
	got := make(chan []SystemLogEntry, 1)
	repo.insertSystemLogsFn = func(ctx context.Context, entries []SystemLogEntry) error {
		got <- entries
		return nil
	}
	inner := &countingHandler{}
	sink := NewLogSink(repo, inner, lv)

	logger := slog.New(sink)
	logger.Info("hello", "component", "scheduler", "request_id", "rid-1", "k", "v")

	if inner.count != 1 {
		t.Errorf("应透传 inner 一次，实际 %d", inner.count)
	}

	// 手动驱动一轮 worker 落库（不起 goroutine，直接读 channel 内容由 worker 处理）。
	// 这里通过启动 Start 后等待一帧的方式验证异步落库。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.Start(ctx)

	select {
	case entries := <-got:
		if len(entries) == 0 {
			t.Fatal("落库条目为空")
		}
		e := entries[0]
		if e.Message != "hello" || e.Component != "scheduler" || e.RequestID != "rid-1" {
			t.Errorf("落库内容不对: %+v", e)
		}
		if e.Level != "info" {
			t.Errorf("级别应为 info，实际 %s", e.Level)
		}
	case <-timeAfter():
		t.Fatal("超时未落库")
	}
}
