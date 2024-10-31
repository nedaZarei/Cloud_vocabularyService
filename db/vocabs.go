package db

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type VocabDB interface {
	addVocab(context.Context, string, string, time.Duration) error
	getVocab(context.Context, string) (string, error)
}

type vocabDBimpl struct {
	redisClient *redis.Client
}

func NewVocabDB(redisClient *redis.Client) VocabDB {
	return &vocabDBimpl{redisClient: redisClient}
}

func (v *vocabDBimpl) addVocab(ctx context.Context, key, value string, expiration time.Duration) error {
	return v.redisClient.Set(ctx, key, value, expiration).Err()
}

func (v *vocabDBimpl) getVocab(ctx context.Context, key string) (string, error) {
	return v.redisClient.Get(ctx, key).Result()
}
