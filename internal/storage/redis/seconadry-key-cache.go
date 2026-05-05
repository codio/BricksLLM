package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type SecondaryKeysCache struct {
	client *redis.Client
	wt     time.Duration
	rt     time.Duration
}

func NewSecondaryKeysCache(c *redis.Client, wt time.Duration, rt time.Duration) *SecondaryKeysCache {
	return &SecondaryKeysCache{
		client: c,
		wt:     wt,
		rt:     rt,
	}
}

func (c *SecondaryKeysCache) Set(sHash string, value string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.wt)
	defer cancel()
	err := c.client.Set(ctx, sHash, value, ttl).Err()
	if err != nil {
		return err
	}

	return nil
}

func (c *SecondaryKeysCache) Delete(sHash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.wt)
	defer cancel()
	err := c.client.Del(ctx, sHash).Err()
	if err != nil {
		return err
	}

	return nil
}

func (c *SecondaryKeysCache) Get(sHash string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.rt)
	defer cancel()

	result := c.client.Get(ctx, sHash)
	err := result.Err()
	if err != nil {
		return "", err
	}

	bs, err := result.Bytes()
	if err != nil {
		return "", err
	}

	return string(bs), nil
}
