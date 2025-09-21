package api

import (
	"context"
	"edgeflow/conf"
	"edgeflow/pkg/logger"
	"edgeflow/pkg/validator"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Router 加载路由，使用侧提供接口，实现侧需要实现该接口
type Router interface {
	Load(engine *gin.Engine)
}

type Server struct {
	config *conf.Config
	f      func()
}

func NewServer(c *conf.Config) *Server {
	return &Server{
		config: c,
	}
}

func (s *Server) Run(rs ...Router) {
	var wg sync.WaitGroup
	wg.Add(1)
	// 设置gin启动模式，必须在创建gin实例之前
	//gin.SetMode(s.config.Mode)
	g := gin.New()
	s.routerLoad(g, rs...)
	// gin validator替换
	validator.LazyInitGinValidator(s.config.Language)

	// health check
	go func() {
		if err := Ping(s.config.Listen, s.config.MaxPingCount); err != nil {
			logger.Fatal("server no response")
		}
		logger.Infof("server started success! port: %s", s.config.Listen)
	}()

	srv := http.Server{
		Addr:    s.config.Listen,
		Handler: g,
	}
	if s.f != nil {
		srv.RegisterOnShutdown(s.f)
	}
	// graceful shutdown
	sgn := make(chan os.Signal, 1)
	signal.Notify(sgn, syscall.SIGINT, syscall.SIGTERM)
	//signal.Notify(sgn, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sgn
		logger.Infof("server shutdown")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Errorf("server shutdown err %v \n", err)
		}
		wg.Done()
	}()

	err := srv.ListenAndServe()
	if err != nil {
		if err != http.ErrServerClosed {
			logger.Errorf("server start failed on port %s", s.config.Listen)
			return
		}
	}
	wg.Wait()
	logger.Infof("server stop on port %s", s.config.Listen)
}

// RouterLoad 加载自定义路由
func (s *Server) routerLoad(g *gin.Engine, rs ...Router) *Server {
	for _, r := range rs {
		r.Load(g)
	}
	return s
}

// RegisterOnShutdown 注册shutdown后的回调处理函数，用于清理资源
func (s *Server) RegisterOnShutdown(_f func()) {
	s.f = _f
}

// Ping 用来检查是否程序正常启动
func Ping(port string, maxCount int) error {
	seconds := 1
	if len(port) == 0 {
		panic("Please specify the service port")
	}
	if !strings.HasPrefix(port, ":") {
		port += ":"
	}
	url := fmt.Sprintf("http://localhost%s/ping", port)
	for i := 0; i < maxCount; i++ {
		resp, err := http.Get(url)
		if nil == err && resp != nil && resp.StatusCode == http.StatusOK {
			return nil
		}
		logger.Infof("等待服务在线, 已等待 %d 秒，最多等待 %d 秒", seconds, maxCount)
		time.Sleep(time.Second * 1)
		seconds++
	}
	return fmt.Errorf("服务启动失败，端口 %s", port)
}
