package policy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Watermark 给每个响应打上可追溯的水印。
//
// 为什么水印有效：技术防护约束的是"能不能拿"，水印约束的是"敢不敢传"。
// 对 B2B 场景后者往往更管用——泄露的截图/文件能反查到账号，
// 比封 IP 更有威慑力，因为它指向具体的人。
func Watermark() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := newTraceID()
		c.Set("trace_id", id)
		c.Header("X-Trace-Id", id)
		// 让调用方知道规则在哪：接口也是对外契约的一部分。
		c.Header("X-Data-Policy", "/api/v1/meta/policy")
		c.Next()
	}
}

func newTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// 取不到随机数也不能让请求失败，退化成时间戳（可追溯性略弱但不断服务）。
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}

// ---------- 限流 ----------

type bucket struct {
	start time.Time
	count int
}

// Limiter 固定窗口计数器。
//
// 复杂度刻意压低：这里的目标不是防 DDoS（那是网关的事），
// 而是拦住"用脚本把全库拖一遍"这种低成本爬取。
// 真实攻击者会换 IP，但那时还有水印、配额与协议约束。
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]*bucket
	sweeps  int
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	if limit <= 0 {
		limit = RequestsPerMin
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{limit: limit, window: window, buckets: make(map[string]*bucket)}
}

// Middleware 按客户端 IP 限流。超出后返回 429 并给出重试等待时间，
// 而不是静默丢弃——静默失败会让正常用户以为系统坏了。
func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			wait := int(l.window.Seconds())
			c.Header("Retry-After", strconv.Itoa(wait))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMITED",
					"message": fmt.Sprintf("请求过于频繁：单个来源每分钟上限 %d 次。如需批量获取数据，请走 API 授权与配额。", l.limit),
				},
			})
			return
		}
		c.Next()
	}
}

func (l *Limiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// 周期性清理过期桶，避免长期运行时 map 无限增长。
	l.sweeps++
	if l.sweeps%2048 == 0 {
		for k, b := range l.buckets {
			if now.Sub(b.start) >= l.window {
				delete(l.buckets, k)
			}
		}
	}

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.buckets[key] = &bucket{start: now, count: 1}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}
