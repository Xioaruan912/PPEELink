package api

import (
	"fmt"
	"log"
	"ppeelink/middlewares"
	"ppeelink/models"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type loginAttempt struct {
	Failures     int
	First        time.Time
	BlockedUntil time.Time
}

var loginAttempts = struct {
	sync.Mutex
	Items map[string]loginAttempt
}{Items: map[string]loginAttempt{}}

const loginWindow = 10 * time.Minute
const loginBlock = 15 * time.Minute
const loginMaxFailures = 5
const loginMaxIPFailures = 20
const loginMaxTrackedKeys = 4096

func loginAttemptKey(c *gin.Context, username string) string {
	return c.ClientIP() + "|" + strings.ToLower(strings.TrimSpace(username))
}
func loginIPKey(c *gin.Context) string {
	return "ip|" + c.ClientIP()
}
func pruneLoginAttemptsLocked(now time.Time) {
	if len(loginAttempts.Items) < 256 {
		return
	}
	for k, v := range loginAttempts.Items {
		if (!v.BlockedUntil.IsZero() && now.After(v.BlockedUntil)) ||
			(v.BlockedUntil.IsZero() && now.Sub(v.First) > loginWindow) {
			delete(loginAttempts.Items, k)
		}
	}
	if len(loginAttempts.Items) >= loginMaxTrackedKeys {
		// Hard cap under heavy spraying: drop oldest non-blocked entries first.
		for k, v := range loginAttempts.Items {
			if v.BlockedUntil.IsZero() {
				delete(loginAttempts.Items, k)
				if len(loginAttempts.Items) < loginMaxTrackedKeys/2 {
					break
				}
			}
		}
	}
}
func loginBlocked(key string) (bool, time.Duration) {
	loginAttempts.Lock()
	defer loginAttempts.Unlock()
	item, ok := loginAttempts.Items[key]
	if !ok {
		return false, 0
	}
	now := time.Now()
	if !item.BlockedUntil.IsZero() && now.Before(item.BlockedUntil) {
		return true, time.Until(item.BlockedUntil)
	}
	if now.Sub(item.First) > loginWindow {
		delete(loginAttempts.Items, key)
		return false, 0
	}
	return false, 0
}
func recordLoginFailureWithLimit(key string, limit int) {
	loginAttempts.Lock()
	defer loginAttempts.Unlock()
	now := time.Now()
	pruneLoginAttemptsLocked(now)
	item := loginAttempts.Items[key]
	if item.First.IsZero() || now.Sub(item.First) > loginWindow {
		item = loginAttempt{First: now}
	}
	item.Failures++
	if item.Failures >= limit {
		item.BlockedUntil = now.Add(loginBlock)
	}
	loginAttempts.Items[key] = item
}
func recordLoginFailure(key string) {
	recordLoginFailureWithLimit(key, loginMaxFailures)
}
func clearLoginFailures(key string) {
	loginAttempts.Lock()
	delete(loginAttempts.Items, key)
	loginAttempts.Unlock()
}

// 获取token
func GetToken(username string) (string, error) {
	// 过期时间天
	ExpireDays := models.ReadConfig().ExpireDays
	c := &middlewares.JwtClaims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24 * time.Duration(ExpireDays)).Unix(), // 设置过期时间
			IssuedAt:  time.Now().Unix(),                                                 // 签发时间
			Subject:   username,                                                          // 用户
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(middlewares.Secret)
}

// 用户登录
func UserLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	attemptKey := loginAttemptKey(c, username)
	ipKey := loginIPKey(c)
	if blocked, remaining := loginBlocked(attemptKey); blocked {
		c.JSON(429, gin.H{"code": "42900", "msg": fmt.Sprintf("登录失败次数过多，请约 %d 分钟后重试", int(remaining.Minutes())+1)})
		return
	}
	if blocked, remaining := loginBlocked(ipKey); blocked {
		c.JSON(429, gin.H{"code": "42900", "msg": fmt.Sprintf("当前 IP 登录尝试过于频繁，请约 %d 分钟后重试", int(remaining.Minutes())+1)})
		return
	}
	user := &models.User{Username: username, Password: password}
	err := user.Verify()
	if err != nil {
		recordLoginFailure(attemptKey)
		recordLoginFailureWithLimit(ipKey, loginMaxIPFailures)
		log.Println("账号或者密码错误")
		c.JSON(400, gin.H{
			"msg": "账号或者密码错误",
		})
		return
	}
	clearLoginFailures(attemptKey)
	// 生成token
	token, err := GetToken(username)
	if err != nil {
		log.Println("获取token失败", err)
		c.JSON(400, gin.H{
			"msg": "获取token失败",
		})
		return
	}
	// 登录成功返回token
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"accessToken":        token,
			"tokenType":          "Bearer",
			"refreshToken":       nil,
			"expires":            nil,
			"mustChangePassword": user.MustChangePassword,
		},
		"msg": "登录成功",
	})
}
func UserOut(c *gin.Context) {
	if raw, ok := c.Get("jwtToken"); ok {
		if tokenStr, isStr := raw.(string); isStr && tokenStr != "" {
			exp := time.Now().Add(24 * time.Hour)
			if rawExp, okExp := c.Get("jwtExp"); okExp {
				if t, isTime := rawExp.(time.Time); isTime {
					exp = t
				}
			}
			middlewares.RevokeToken(tokenStr, exp)
		}
	}
	if _, Is := c.Get("username"); Is {
		c.JSON(200, gin.H{
			"code": "00000",
			"msg":  "退出成功",
		})
	}
}
