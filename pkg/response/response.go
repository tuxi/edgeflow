package response

import (
	"edgeflow/internal/consts"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"github.com/gin-gonic/gin"
	"net/http"
)

// 代表响应给客户端的的一个消息结构，包括错误码，错误信息，响应数据
type ApiResponse struct {
	RequestId   string      `json:"request_id"`   // 请求的唯一ID
	Code        int         `json:"code"`         // 错误码 0表示无错误
	Message     string      `json:"message"`      // 提示信息
	Data        interface{} `json:"data"`         // 响应数据，一般从这里前端从这个里面取出数据展示
	IsEncrypted bool        `json:"is_encrypted"` // 是否是加密的
}

// 发送json格式数据
func JSONEncrypted(c *gin.Context, err error, encryptedData interface{}) {
	code, message := errors.DecodeErr(err)
	// 如果code != 0, 失败的话 返回http状态码400（一般也可以全部返回200）
	// 返回400 更严谨一些，个人接触的项目中大部分都是400。
	var httpStatus int
	if code != ecode.Success {
		httpStatus = http.StatusBadRequest
	} else {
		httpStatus = http.StatusOK
	}
	c.JSON(httpStatus, ApiResponse{
		RequestId:   c.GetString(consts.RequestId),
		Code:        code,
		Message:     message,
		Data:        encryptedData,
		IsEncrypted: true,
	})
}

// 发送json格式数据
func JSON(c *gin.Context, err error, data interface{}) {
	code, message := errors.DecodeErr(err)
	// 如果code != 0, 失败的话 返回http状态码400（一般也可以全部返回200）
	// 返回400 更严谨一些，个人接触的项目中大部分都是400。
	var httpStatus int
	if code != ecode.Success {
		httpStatus = http.StatusBadRequest
	} else {
		httpStatus = http.StatusOK
	}
	c.JSON(httpStatus, ApiResponse{
		RequestId: c.GetString(consts.RequestId),
		Code:      code,
		Message:   message,
		Data:      data,
	})
}

// token鉴权失败，返回401
func RequireAuthErr(c *gin.Context, err error) {
	var message string
	if err != nil {
		message = err.Error()
	} else {
		message = "unknow error."
	}
	c.JSON(http.StatusUnauthorized, ApiResponse{
		RequestId: c.GetString(consts.RequestId),
		Code:      ecode.RequireAuthErr,
		Message:   "invalid token:" + message,
		Data:      nil,
	})
}

// 请求频繁，返回429
func TooManyRequests(c *gin.Context) {
	message := "The request is too frequent. Please try again later."
	c.JSON(http.StatusTooManyRequests, ApiResponse{
		RequestId: c.GetString(consts.RequestId),
		Code:      ecode.RequireAuthErr,
		Message:   "invalid token:" + message,
		Data:      nil,
	})
}

// 请求频繁，返回400
func BadRequests(c *gin.Context) {
	message := "Invalid request, missing signature."
	c.JSON(http.StatusBadRequest, ApiResponse{
		RequestId: c.GetString(consts.RequestId),
		Code:      ecode.RequireAuthErr,
		Message:   "invalid token:" + message,
		Data:      nil,
	})
}
