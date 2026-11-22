package middlewares

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ipBucket struct {
	tokens float64
	last   time.Time
}

var publicLimiter = struct {
	sync.Mutex
	buckets map[string]*ipBucket
}{buckets: map[string]*ipBucket{}}

const (
	publicRatePerSec = 1.0 // 60 req/min sustained per IP
	publicBurst      = 25.0
	publicMaxBuckets = 4096
)

var publicLimitedPrefixes = []string{"/c/", "/cc/", "/s/", "/t/", "/p/"}

func isPublicLimitedPath(path string) bool {
	for _, p := range publicLimitedPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// PublicRateLimit protects unauthenticated subscription and shortlink endpoints
// from abusive scraping bursts using a bounded per-IP token bucket.
func PublicRateLimit(c *gin.Context) {
	if !isPublicLimitedPath(c.Request.URL.Path) {
		c.Next()
		return
	}
	if !allowPublicIP(c.ClientIP(), time.Now()) {
		c.Header("Retry-After", "10")
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "42900", "msg": "请求过于频繁，请稍后再试"})
		c.Abort()
		return
	}
	c.Next()
}

func allowPublicIP(ip string, now time.Time) bool {
	publicLimiter.Lock()
	defer publicLimiter.Unlock()
	if len(publicLimiter.buckets) >= 512 {
		for k, b := range publicLimiter.buckets {
			if now.Sub(b.last) > 5*time.Minute {
				delete(publicLimiter.buckets, k)
			}
		}
		if len(publicLimiter.buckets) >= publicMaxBuckets {
			for k := range publicLimiter.buckets {
				delete(publicLimiter.buckets, k)
				if len(publicLimiter.buckets) < publicMaxBuckets/2 {
					break
				}
			}
		}
	}
	b, ok := publicLimiter.buckets[ip]
	if !ok {
		publicLimiter.buckets[ip] = &ipBucket{tokens: publicBurst - 1, last: now}
		return true
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * publicRatePerSec
		if b.tokens > publicBurst {
			b.tokens = publicBurst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}
