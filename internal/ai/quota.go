package ai

import (
	"context"
	"fmt"
	"time"

	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

type quotaStore struct {
	rdb   *platformredis.Client
	repo  *repository
	limit int
	loc   *time.Location
}

func newQuotaStore(rdb *platformredis.Client, repo *repository, limit int) *quotaStore {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	if limit <= 0 {
		limit = 50
	}
	return &quotaStore{rdb: rdb, repo: repo, limit: limit, loc: loc}
}

func (q *quotaStore) shanghaiDay(now time.Time) string {
	return now.In(q.loc).Format("2006-01-02")
}

func (q *quotaStore) Take(ctx context.Context, userID int64) error {
	day := q.shanghaiDay(time.Now())
	if q.rdb != nil {
		key := fmt.Sprintf("ai:quota:%d:%s", userID, day)
		n, err := q.rdb.Incr(ctx, key).Result()
		if err != nil {
			// fall through to MySQL
		} else {
			if n == 1 {
				// expire slightly after next Shanghai midnight
				now := time.Now().In(q.loc)
				next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, q.loc)
				_ = q.rdb.ExpireAt(ctx, key, next).Err()
			}
			if int(n) > q.limit {
				_, _ = q.rdb.Decr(ctx, key).Result()
				return ErrQuotaExceeded
			}
			return nil
		}
	}
	if q.repo == nil {
		return nil
	}
	_, err := q.repo.IncrUsageDaily(ctx, userID, day, q.limit)
	return err
}
