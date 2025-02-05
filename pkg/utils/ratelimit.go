package utils

import (
	"math"
	"time"

	"github.com/juju/ratelimit"
)

type TokenBucket struct {
	Id              string
	bucket          *ratelimit.Bucket
	capacity        int64
	waitMaxDuration time.Duration
}

func NewTokenBucket(id string, rate float64) *TokenBucket {
	if rate == 0 {
		rate = math.MaxUint64
	}
	capacity := int64(1)
	bucket := ratelimit.NewBucketWithRate(rate, capacity)
	return &TokenBucket{
		Id:       id,
		bucket:   bucket,
		capacity: capacity,
	}
}

func (tb *TokenBucket) WithWaitMaxDuration(duration time.Duration) *TokenBucket {
	tb.waitMaxDuration = duration
	return tb
}

func (tb *TokenBucket) Take() bool {
	if tb.waitMaxDuration == 0 {
		return tb.bucket.TakeAvailable(1) > 0
	}
	return tb.bucket.WaitMaxDuration(1, tb.waitMaxDuration)
}

func (tb *TokenBucket) Filled() bool {
	return tb.bucket.Available() == tb.capacity
}
