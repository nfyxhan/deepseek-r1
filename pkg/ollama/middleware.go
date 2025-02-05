package ollama

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nfyxhan/deepseek-r1/pkg/utils"
)

var cache = &utils.LRUCache{}
var sm = &sync.Map{}

func init() {
	cache.Init(func(e *utils.Element) bool {
		tb, _ := e.Data.(*utils.TokenBucket)
		return tb.Filled()
	}, 1000)
}

func SetQps(id string, qps float64) {
	sm.Store(id, qps)
}
func GetQps(id string) float64 {
	r, ok := sm.Load(id)
	if !ok {
		return -1
	}
	qps, _ := r.(float64)
	return qps
}

func newBucket(id string, qps float64, waitMaxDuration time.Duration) *utils.TokenBucket {
	key := fmt.Sprintf("%s_%v", id, qps)
	if e := cache.Get(key); e != nil {
		tb, _ := e.Data.(*utils.TokenBucket)
		return tb
	}
	tb := utils.NewTokenBucket(id, qps)
	// if waitMaxDuration is not 0, it will block if a available token will generate in waitMaxDuration
	// else, it will return if it has no token available
	if waitMaxDuration > 0 {
		tb = tb.WithWaitMaxDuration(waitMaxDuration)
	}
	if err := cache.Add(key, tb); err == nil {
		return tb
	}
	if e := cache.Get(key); e != nil {
		tb, _ = e.Data.(*utils.TokenBucket)
	}
	return tb
}

type LimitByFunc func(c *gin.Context) (string, bool)

func LimitByClientIP(id string) func(c *gin.Context) (string, bool) {
	return func(c *gin.Context) (string, bool) {
		return fmt.Sprintf("%s_cip_%s", id, c.ClientIP()), true
	}
}

func LimitByRequestMethod(id string, methods []interface{}) func(c *gin.Context) (string, bool) {
	return func(c *gin.Context) (string, bool) {
		method := c.Request.Method
		limit := false
		for _, m := range methods {
			if m == method {
				limit = true
				break
			}
		}
		return fmt.Sprintf("%s_method", id), limit
	}
}

// qps is the max number of the tokens it can take in 1 second
// waitMaxDuration is the max wait duration if it has no token for now
// ps: qps * waitMaxDuration / time.Second is the max concurrency number of objects who taking a token
func RateLimit(limitBy LimitByFunc, qps float64, waitMaxDuration time.Duration) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		id, ok := limitBy(ctx)
		q := GetQps(id)
		if q > 0 {
			qps = q
		}
		if ok && !newBucket(id, qps, waitMaxDuration).Take() {
			log.Printf("rate limit exceed: %s %s", ctx.Request.RemoteAddr, ctx.Request.URL.Path)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
