package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bricks-cloud/bricksllm/internal/event"
	"github.com/redis/go-redis/v9"
)

const inProgressKeyPrefix = "statistic_in_progress_"

type StatisticCache struct {
	client *redis.Client
	wt     time.Duration
	rt     time.Duration
}

func NewStatisticCache(c *redis.Client, wt time.Duration, rt time.Duration) *StatisticCache {
	return &StatisticCache{
		client: c,
		wt:     wt,
		rt:     rt,
	}
}

func (c *StatisticCache) Set(key string, value *event.StatisticsData, ttl time.Duration) error {
	if value == nil {
		return errors.New("statistic data to set is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.wt)
	defer cancel()
	bs, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = c.client.Set(ctx, key, bs, ttl).Err()
	if err != nil {
		return err
	}

	return nil
}

func (c *StatisticCache) Delete(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.wt)
	defer cancel()
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	return nil
}

func (c *StatisticCache) Get(key string) (*event.StatisticsData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.rt)
	defer cancel()

	result := c.client.Get(ctx, key)
	err := result.Err()
	if err != nil {
		return nil, err
	}

	bs, err := result.Bytes()
	if err != nil {
		return nil, err
	}

	var stat event.StatisticsData
	err = json.Unmarshal(bs, &stat)
	if err != nil {
		return nil, err
	}

	return &stat, nil
}

func (c *StatisticCache) SetInProgress(key string) error {
	k := inProgressKeyPrefix + key
	ctx, cancel := context.WithTimeout(context.Background(), c.wt)
	defer cancel()
	err := c.client.Set(ctx, k, true, time.Minute*10).Err()
	if err != nil {
		return err
	}
	return nil
}

func (c *StatisticCache) IsInProgress(key string) bool {
	k := inProgressKeyPrefix + key
	ctx, cancel := context.WithTimeout(context.Background(), c.rt)
	defer cancel()
	return c.client.Get(ctx, k).Err() == nil
}

func (c *StatisticCache) DeleteInProgress(key string) error {
	k := inProgressKeyPrefix + key
	ctx, cancel := context.WithTimeout(context.Background(), c.wt)
	defer cancel()
	err := c.client.Del(ctx, k).Err()
	if err != nil {
		return err
	}
	return nil
}
