package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	jwtsvc "github.com/sameetpatro/go-qr-auth/internal/auth"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/pkg/response"
)

const (
	ContextUserIDKey = "user_id"
	ContextEmailKey  = "email"
	ContextRoleKey   = "role"
)

func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP, param.TimeStamp.Format(time.RFC1123),
			param.Method, param.Path, param.Request.Proto,
			param.StatusCode, param.Latency, param.Request.UserAgent(), param.ErrorMessage,
		)
	})
}

func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

		if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			for _, o := range allowedOrigins {
				if o == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func RateLimit(rpm int) gin.HandlerFunc {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    rpm,
		window:   time.Minute,
	}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		rl.mu.Lock()
		times := rl.requests[ip]
		var valid []time.Time
		for _, t := range times {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		if len(valid) >= rl.limit {
			rl.mu.Unlock()
			response.Error(c, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
			return
		}
		valid = append(valid, now)
		rl.requests[ip] = valid
		rl.mu.Unlock()
		c.Next()
	}
}

func JWTAuth(jwtService *jwtsvc.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c, "Missing or invalid authorization header")
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtService.ValidateAccessToken(token)
		if err != nil {
			response.Unauthorized(c, "Invalid or expired token")
			return
		}
		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextEmailKey, claims.Email)
		c.Set(ContextRoleKey, string(claims.Role))
		c.Next()
	}
}

func RequireRole(roles ...models.UserRole) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[string(r)] = true
	}
	return func(c *gin.Context) {
		role, exists := c.Get(ContextRoleKey)
		if !exists || !allowed[role.(string)] {
			response.Forbidden(c, "Insufficient permissions")
			return
		}
		c.Next()
	}
}

func DenyRole(roles ...models.UserRole) gin.HandlerFunc {
	denied := make(map[string]bool, len(roles))
	for _, r := range roles {
		denied[string(r)] = true
	}
	return func(c *gin.Context) {
		role, exists := c.Get(ContextRoleKey)
		if exists && denied[role.(string)] {
			response.Forbidden(c, "Insufficient permissions")
			return
		}
		c.Next()
	}
}

func GetUserID(c *gin.Context) int64 {
	return c.GetInt64(ContextUserIDKey)
}

func GetRole(c *gin.Context) models.UserRole {
	return models.UserRole(c.GetString(ContextRoleKey))
}

func GetClientIP(c *gin.Context) string {
	return c.ClientIP()
}
