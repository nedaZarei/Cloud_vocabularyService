package db

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type VocabDB interface {
	AddVocab(context.Context, string, string, time.Duration) error
	GetVocab(context.Context, string) (string, error)
}

type vocabDBimpl struct {
	redisClient *redis.Client
}

func NewVocabDB(redisClient *redis.Client) VocabDB {
	return &vocabDBimpl{redisClient: redisClient}
}

func (v *vocabDBimpl) AddVocab(ctx context.Context, key, value string, expiration time.Duration) error {
	return v.redisClient.Set(ctx, key, value, expiration).Err()
}

func (v *vocabDBimpl) GetVocab(ctx context.Context, key string) (string, error) {
	return v.redisClient.Get(ctx, key).Result()
}

func InitRedisClient(host, port string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: host + ":" + port,
	})
}
