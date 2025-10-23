package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"edgeflow/internal/consts"
	"edgeflow/pkg/response"
	"edgeflow/utils/uuid"
	"encoding/base64"
	lru "github.com/hashicorp/golang-lru"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// NoCache 控制客户端不要使用缓存
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, max-age=0, must-revalidate")
		c.Header("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
		c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		c.Next()
	}
}

// Options
func Options() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.ToUpper(c.Request.Method) != "OPTIONS" {
			c.Next()
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "authorization, origin, content-type, accept")
			c.Header("Allow", "HEAD,GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Content-State", "application/json")
			c.AbortWithStatus(http.StatusOK)
		}
	}
}

func Stream() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-State", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")
		c.Next()
	}
}

// Secure 添加安全控制和资源访问
func Secure() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-State-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000")
		}
		c.Next()
	}
}

// RequestId 用来设置和透传requestId
func RequestId() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestId := uuid.GenUUID16()
		c.Header("X-Request-Id", requestId)

		// 设置requestId到context中，便于后面调用链的透传
		c.Set(consts.RequestId, requestId)
		c.Next()
	}
}

func ApiBaseHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		//headers := c.Request.Header
		//logger.Infof("headers: %s", headers)

		clientId := c.Request.Header.Get(consts.ClientId)
		c.Set(consts.ClientId, clientId)

		clientVersion := c.Request.Header.Get(consts.ClientVersion)
		c.Set(consts.ClientVersion, clientVersion)

		// 设置设备id
		deviceId := c.Request.Header.Get(consts.DeviceId)
		c.Set(consts.DeviceId, deviceId)

		c.Next()
	}
}

// 使用分片锁 (Sharding/Bucket Locking)，降低锁的粒度
// 不能使用sync.Mutex，会导致阻塞

// 限制缓存的最大大小为 500，且是并发安全的 LRU 缓存
var reqCache, _ = lru.New(500)
var duplicateThreshold = 1 * time.Second

// 定义一个中间件函数，用于防止频繁请求和重复提交
// 防止单个客户端 IP 在 1 秒内重复发送请求，保护下游 API 或系统资源
// 将 AntiDuplicateMiddleware 只应用于不需要高频重试的 常规 HTTP API 路由，例如获取配置、登录等，不应该用于websocket等实时性高的连接
func AntiDuplicateMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + c.Request.URL.Path
		// 使用IP + 接口路径 作为key 防抖动
		// 使用golang-lru 缓存库，解决锁竞争问题
		// 检查记录是否存在
		if value, ok := reqCache.Get(key); ok { // 注意：使用 key
			lastRequestTime := value.(time.Time)

			// 检查是否在阀值内
			if time.Since(lastRequestTime) < duplicateThreshold {
				// 如果是重复请求，直接返回
				response.TooManyRequests(c)
				c.Abort()
				return
			}
		}

		// 更新时间戳 (Hit 或 Miss 都会更新)
		// Set 方法会自动处理 LRU 淘汰和并发安全
		reqCache.Add(key, time.Now())
		c.Next()
	}
}

func RequestValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientVersion := c.GetString(consts.ClientVersion)
		if clientVersion < "1.0.3" {
			c.Next()
			return
		}
		timestamp := c.GetHeader(consts.Timestamp) // 客户端在请求头中添加时间戳
		signature := c.GetHeader(consts.Signature) // 客户端在请求头中添加签名

		// 验证时间戳的有效性

		// 将UTC时间戳字符串转换为时间
		utcTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			response.BadRequests(c)
			c.Abort()
			return
		}

		// 获取当前UTC时间戳
		currentUTCTimestamp := time.Now().Unix()

		// 设置一个时间阈值，例如，1分钟
		timeThreshold := int64(1 * time.Minute)

		// 验证时间戳是否在阈值内
		if (currentUTCTimestamp - utcTimestamp) > timeThreshold {
			response.BadRequests(c)
			c.Abort()
			return
		}

		// 验证签名
		validSignature := computeHMAC(timestamp, []byte(consts.RequestSecretKey))
		if signature != validSignature {
			// 无效的签名。
			response.BadRequests(c)
			c.Abort()
			return
		}

		// 继续执行下一个处理程序
		c.Next()
	}
}

func computeHMAC(data string, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
