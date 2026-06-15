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

// ListLogs 分页查询审核日志。
func (s *ModerationStore) ListLogs(ctx context.Context, f appmoderation.LogFilter) (appmoderation.LogResult, error) {
	q := s.db.ModerationLog.Query()
	if f.UserID > 0 {
		q = q.Where(entmodlog.UserIDSnapshotEQ(f.UserID))
	}
	if f.Platform != "" {
		q = q.Where(entmodlog.PlatformEQ(f.Platform))
	}
	if f.Flagged != nil {
		q = q.Where(entmodlog.FlaggedEQ(*f.Flagged))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return appmoderation.LogResult{}, err
	}

	rows, err := q.
		Order(ent.Desc(entmodlog.FieldCreatedAt)).
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		All(ctx)
	if err != nil {
		return appmoderation.LogResult{}, err
	}

	list := make([]appmoderation.LogRecord, 0, len(rows))
	for _, r := range rows {
		list = append(list, appmoderation.LogRecord{
			ID:        r.ID,
			RequestID: r.RequestID,
			UserID:    r.UserIDSnapshot,
			Platform:  r.Platform,
			Endpoint:  r.Endpoint,
			Mode:      r.Mode,
			Flagged:   r.Flagged,
			Source:    r.Source,
			Category:  r.Category,
			Score:     r.Score,
			Excerpt:   r.Excerpt,
			CreatedAt: r.CreatedAt,
		})
	}
	return appmoderation.LogResult{List: list, Total: int64(total), Page: f.Page, PageSize: f.PageSize}, nil
}
