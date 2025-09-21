package middleware

import (
	"edgeflow/internal/handler/ping"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
)

// middleware 实现Router接口
// 便于服务启动时加载, middleware本质跟handler无区别
type middleware struct {
}

func NewMiddleware() *middleware {
	return &middleware{}
}

// Load 注册中间件和公共路由
func (m *middleware) Load(g *gin.Engine) {
	// 注册中间件
	g.Use(gin.Logger())
	g.Use(gin.Recovery())
	g.Use(AntiDuplicateMiddleware())
	g.Use(NoCache())
	g.Use(Options())
	g.Use(Secure())
	g.Use(RequestId())
	g.Use(ApiBaseHeader())
	g.Use(Logger)
	// 404
	g.NoRoute(func(c *gin.Context) {
		response.JSON(c, errors.WithCode(ecode.NotFoundErr, "404 not found!"), nil)
	})
	// ping server
	g.GET("/ping", ping.Ping())
}
