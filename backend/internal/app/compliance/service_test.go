package compliance

import (
	"context"
	"testing"

	settings "github.com/DouDOU-start/airgate-core/internal/app/settings"
)

// fakeSettings 内存版 SettingsPort，用于测试。
type fakeSettings struct {
	data    map[string]settings.Setting // key -> setting
	listErr error
}

func newFakeSettings() *fakeSettings {
	return &fakeSettings{data: make(map[string]settings.Setting)}
}

func (f *fakeSettings) List(_ context.Context, group string) ([]settings.Setting, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]settings.Setting, 0, len(f.data))
	for _, s := range f.data {
		if group == "" || s.Group == group {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSettings) Update(_ context.Context, items []settings.ItemInput) error {
	for _, it := range items {
		f.data[it.Key] = settings.Setting{Key: it.Key, Value: it.Value, Group: it.Group}
	}
	return nil
}

func TestService_Status_GateDisabledNeverRequired(t *testing.T) {
	fs := newFakeSettings()
	svc := NewService(fs)

	st, err := svc.Status(context.Background(), 7, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if st.Required {
		t.Errorf("门关闭时 Required 应为 false")
	}
	if st.Enabled {
		t.Errorf("默认门应关闭")
	}
	if st.Phrase != phraseZh {
		t.Errorf("默认语言短语应为中文，得 %q", st.Phrase)
	}
	if st.DocumentZh == "" || st.DocumentEn == "" {
		t.Errorf("文档全文不应为空（embed 失败？）")
	}
}

func TestService_Status_EnabledRequiresAck(t *testing.T) {
	fs := newFakeSettings()
	_ = fs.Update(context.Background(), []settings.ItemInput{
		{Key: keyGateEnabled, Value: "true", Group: settingGroup},
	})
	svc := NewService(fs)

	st, err := svc.Status(context.Background(), 7, "en")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !st.Enabled {
		t.Fatalf("门应开启")
	}
	if !st.Required {
		t.Errorf("开启且未确认时 Required 应为 true")
	}
	if st.Phrase != phraseEn {
		t.Errorf("language=en 应返回英文短语")
	}
}

func TestService_Accept_WrongPhrase(t *testing.T) {
	fs := newFakeSettings()
	_ = fs.Update(context.Background(), []settings.ItemInput{
		{Key: keyGateEnabled, Value: "true", Group: settingGroup},
	})
	svc := NewService(fs)

	_, err := svc.Accept(context.Background(), AcceptInput{
		AdminUserID: 7, Phrase: "瞎写的短语", Language: "zh",
	})
	if err != ErrInvalidPhrase {
		t.Errorf("错误短语应返回 ErrInvalidPhrase，得 %v", err)
	}
}

func TestService_Accept_ThenAcknowledged(t *testing.T) {
	fs := newFakeSettings()
	_ = fs.Update(context.Background(), []settings.ItemInput{
		{Key: keyGateEnabled, Value: "true", Group: settingGroup},
	})
	svc := NewService(fs)
	ctx := context.Background()

	st, err := svc.Accept(ctx, AcceptInput{
		AdminUserID: 7, Phrase: phraseZh, Language: "zh",
		IPAddress: "1.2.3.4", UserAgent: "ut",
	})
	if err != nil {
		t.Fatalf("正确短语不应报错: %v", err)
	}
	if st.Required {
		t.Errorf("确认后 Required 应为 false")
	}
	if !st.Acknowledged || st.Ack == nil {
		t.Fatalf("确认后应 Acknowledged 且带 Ack")
	}
	if st.Ack.Version != Version || st.Ack.AdminUserID != 7 || st.Ack.IPAddress != "1.2.3.4" {
		t.Errorf("确认记录字段不符: %+v", st.Ack)
	}

	// 再查一次：另一管理员(8)不受 7 的确认影响，仍需确认
	st8, _ := svc.Status(ctx, 8, "")
	if !st8.Required {
		t.Errorf("管理员 8 未确认，应仍 Required")
	}

	// Gate 判定一致
	blocked7, _, _ := svc.Gate(ctx, 7)
	blocked8, _, _ := svc.Gate(ctx, 8)
	if blocked7 {
		t.Errorf("管理员 7 已确认，Gate 不应拦截")
	}
	if !blocked8 {
		t.Errorf("管理员 8 未确认，Gate 应拦截")
	}
}

func TestService_Gate_FailOpenOnError(t *testing.T) {
	fs := newFakeSettings()
	fs.listErr = context.DeadlineExceeded
	svc := NewService(fs)

	blocked, version, err := svc.Gate(context.Background(), 7)
	if err == nil {
		t.Errorf("应回传底层错误供中间件记录")
	}
	if blocked {
		t.Errorf("读设置失败时应 fail-open（不拦截）")
	}
	if version != Version {
		t.Errorf("降级时仍应返回当前版本号")
	}
}
