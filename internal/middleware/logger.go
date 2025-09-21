package middleware

import (
	"bytes"
	"edgeflow/internal/consts"
	"edgeflow/pkg/logger"
	"github.com/gin-gonic/gin"
	"io"
	"time"
)

func Logger(c *gin.Context) {
	// 请求前
	t := time.Now()
	reqPath := c.Request.URL.Path
	reqId := c.GetString(consts.RequestId)
	method := c.Request.Method
	ip := c.ClientIP()
	requestBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		requestBody = []byte{}
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))

	logger.Info("[Request Start]",
		logger.Pair(consts.RequestId, reqId),
		logger.Pair("host", ip),
		logger.Pair("host", ip),
		logger.Pair("path", reqPath),
		logger.Pair("method", method),
		logger.Pair("body", string(requestBody)))

	c.Next()
	// 请求后
	latency := time.Since(t)
	logger.Info("[Request End]",
		logger.Pair(consts.RequestId, reqId),
		logger.Pair("host", ip),
		logger.Pair("path", reqPath),
		logger.Pair("cost", latency))
}
