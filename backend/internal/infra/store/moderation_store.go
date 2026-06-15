package store

import (
	"context"
	"time"

	"github.com/DouDOU-start/airgate-core/ent"
	entmodlog "github.com/DouDOU-start/airgate-core/ent/moderationlog"
	appmoderation "github.com/DouDOU-start/airgate-core/internal/app/moderation"
)

// ModerationStore 使用 Ent 实现内容审核域仓储。仅本层 import ent。
type ModerationStore struct {
	db *ent.Client
}

// NewModerationStore 创建审核仓储。
func NewModerationStore(db *ent.Client) *ModerationStore {
	return &ModerationStore{db: db}
}

// 编译期接口断言。
var _ appmoderation.Repository = (*ModerationStore)(nil)

// InsertLog 落一条审核命中日志。
func (s *ModerationStore) InsertLog(ctx context.Context, in appmoderation.LogInput) error {
	createdAt := in.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := s.db.ModerationLog.Create().
		SetRequestID(in.RequestID).
		SetUserIDSnapshot(in.UserID).
		SetPlatform(in.Platform).
		SetEndpoint(in.Endpoint).
		SetMode(string(in.Mode)).
		SetFlagged(in.Flagged).
		SetSource(string(in.Source)).
		SetCategory(in.Category).
		SetScore(in.Score).
		SetExcerpt(in.Excerpt).
		SetCreatedAt(createdAt).
		Save(ctx)
	return err
}

// CountViolationsSince 统计某用户 since 之后的命中条数（封禁判定用）。
func (s *ModerationStore) CountViolationsSince(ctx context.Context, userID int, since time.Time) (int, error) {
	return s.db.ModerationLog.Query().
		Where(
			entmodlog.UserIDSnapshotEQ(userID),
			entmodlog.FlaggedEQ(true),
			entmodlog.CreatedAtGTE(since),
		).
		Count(ctx)
}
