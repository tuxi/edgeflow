package user

import (
	"edgeflow/internal/consts"
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/logger"
	"edgeflow/pkg/response"
	"edgeflow/utils/security"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"strings"
)

type UserHandler struct {
	service service.UserService
	ds      service.DeviceService
}

func NewUserHandler(service service.UserService, ds service.DeviceService) *UserHandler {
	return &UserHandler{service: service, ds: ds}
}

// @Summary		获取用户头像
// @title			Chat API
// @version		1.0
// @description	用户头像的base64数据
// @Produce		json
// @Param			Authorization	header		string	false	"Bearer 用户令牌"
// @Success		200				{object}	response.ApiResponse{data=model.UserAvatarRes}
// @Router			/api/v1/user/avatar [get]
func (handler *UserHandler) UserGetAvatar() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := handler.service.UserGetAvatar(ctx)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
			return
		}
		response.JSON(ctx, nil, res)
	}
}

// @Summary		获取用户余额
// @title			Chat API
// @version		1.0
// @description	用来获取用户余额的
// @Produce		json
// @Param			Authorization	header		string								false	"Bearer 用户令牌"
// @Success		200				{object}	response.ApiResponse{data=model.UserBalanceGetRes}	"返回的是具体的余额"
// @Router			/api/v1/user/balance [get]
func (handler *UserHandler) UserGetBalance() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userId := ctx.GetInt64(consts.UserID)
		res, err := handler.service.UserGetBalance(ctx, userId)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
			return
		}

		response.JSON(ctx, nil, res)
	}
}

// @Summary		获取用户计划，包含余额
// @title			Chat API
// @version		1.0
// @description	用来获取用户余额和订阅信息
// @Produce		json
// @Param			Authorization	header		string								false	"Bearer 用户令牌"
// @Success		200				{object}	response.ApiResponse{data=model.UserPlanGetRes}	"返回的是用户余额和订阅"
// @Router			/api/v1/user/plan [get]
func (handler *UserHandler) UserGetPlan() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userId := ctx.GetInt64(consts.UserID)
		res, err := handler.service.UserGetPlan(ctx, userId)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
			return
		}
		encryptData, err := handler.ds.UserDeviceEncrypt(ctx, res)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.ValidateErr, "Encrypt data error."), nil)
			return
		}
		response.JSONEncrypted(ctx, nil, encryptData)
	}
}

// @Summary		获取用户详情
// @title			Chat API
// @version		1.0
// @description	用来获取当前登陆用户的详细信息
// @Produce		json
// @Param			Authorization	header		string	false	"Bearer 用户令牌"
// @Success		200				{object}	response.ApiResponse{data=model.UserGetInfoRes}
// @Router			/api/v1/user/info [get]
func (handler *UserHandler) UserGetInfo() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userId := ctx.GetInt64(consts.UserID)
		res, err := handler.service.UserGetInfo(ctx, userId)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.NotFoundErr, "未找到用户信息"), nil)
			return
		}
		encryptData, err := handler.ds.UserDeviceEncrypt(ctx, res)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.ValidateErr, "Encrypt data error."), nil)
			return
		}
		response.JSONEncrypted(ctx, nil, encryptData)
	}
}

// @Summary		用户注册接口
// @title			Swagger API
// @version		1.0
// @description	用户注册接口
// @Accept			json
// @Produce		json
// @Param			username	body		string	true	"用户名"
// @Param			password	body		string	true	"密码 加密后的"
// @Param			email		body		string	true	"邮箱"
// @Param			captcha		body		string	true	"验证码"
// @Param			invite_code	body		string	false	"邀请码"
// @Success		200			{object}	response.ApiResponse{data=model.UserRegisterRes}
// @Router			/api/v1/auth/register [post]
func (handler *UserHandler) UserRegister() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 读取请求参数
		var req model.UserRegisterReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "请求参数错误"), nil)
			return
		}

		// 验证验证码是否正确
		isCode := handler.service.CaptchaVerify(ctx, req.Captcha)
		if !isCode {
			response.JSON(ctx, errors.WithCode(ecode.CaptchaErr, "验证码错误"), nil)
			return
		}
		r1, err := handler.service.UserVerifyUserName(ctx, req.Username)
		r2, err := handler.service.UserVerifyEmail(ctx, req.Email)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.Unknown, "邮箱地址错误或无法接受邮件"), nil)
			logger.Error(err.Error())
			return
		}
		if !r1.IsValid || !r2.IsValid {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "用户名或邮箱已被注册"), nil)
			logger.Error(err.Error())
			return
		}
		// 注册
		res, err := handler.service.UserRegister(ctx, req)
		if err != nil {
			// 发生错误，删除用户
			err = handler.service.UserDelete(ctx)
			if err != nil {
				logger.Errorf("注册失败后，删除用户失败：%v", err.Error())
			}
			response.JSON(ctx, errors.Wrapf(err, ecode.Unknown, "发生错误注册失败"), nil)
			logger.Error(err.Error())
			return
		}

		// 生成激活码
		err = handler.service.UserActiveGen(ctx)
		if err != nil {
			_ = handler.service.UserDelete(ctx)
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "未知错误注册失败"), res)
			logger.Error(err.Error())
			return
		}

		// 验证邀请码
		handler.service.UserInviteVerify(ctx, req.InviteCode)
		_ = handler.service.UserLogSave(ctx, "register", "success")
		response.JSON(ctx, errors.Wrapf(err, ecode.Success, "注册成功，激活链接已发送到您的邮箱 %s 。", req.Email), res)
	}
}

// @Summary		用户登陆
// @Description	用户密码登陆
// @Accept			json
// @Produce		json
// @Param			username	body		string	true	"用户名"
// @Param			password	body		string	true	"密码"
// @Param			captcha		body		string	true	"验证码"
// @Success		200			{object}	response.ApiResponse{data=model.UserLoginRes}
// @Failure		404			{object}	nil
// @Router			/api/v1/auth/login [post]
func (handler *UserHandler) UserLogin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserLoginReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		isCode := handler.service.CaptchaVerify(ctx, req.Captcha)
		if !isCode {
			response.JSON(ctx, errors.WithCode(ecode.CaptchaErr, "验证码错误"), nil)
			return
		}

		res, err := handler.service.UserLogin(ctx, req.Username, req.Password)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.UserLoginErr, "登陆失败：账号或密码错误"), nil)
			return
		}
		_ = handler.service.UserLogSave(ctx, "password_login", "登陆成功")
		response.JSON(ctx, errors.Wrap(err, ecode.Success, "登陆成功"), res)
	}
}

// @Summary		获取一个匿名用户的访问token
// @title			Chat api
// @version		1.0
// @description	获取一个匿名用户的访问token
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.GetAnonymousAccessTokenReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse
// @Router			/api/v1/auth/anonymous/accessToken [post]
func (handler *UserHandler) GetAnonymousAccessToken() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var req model.GetAnonymousAccessTokenReq
		if err := ctx.ShouldBindBodyWith(&req, binding.JSON); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "缺少必须的参数"), nil)
			return
		}
		// 生成token
		res, isNew, err := handler.service.GetAnonymousAccessToken(ctx, req)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.UserLoginErr, "匿名用户获取access token失败"), nil)
			return
		}
		res.IsNew = isNew
		response.JSON(ctx, errors.Wrap(err, ecode.Success, "获取access token成功"), res)
	}
}

// @Summary		用户退出登陆
// @title			Chat API
// @version		1.0
// @description	用户退出登陆时可调用此api，用来将用户token失效的
// @Produce		json
// @Param			Authorization	header		string	false	"Bearer 用户令牌"
// @Success		200				{object}	response.ApiResponse
// @Router			/api/v1/user/logout [get]
func (handler *UserHandler) UserLogout() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenStr := ctx.GetString(consts.JWTTokenCtx)
		err := handler.service.UserLogout(ctx, tokenStr)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.UserLoginErr, "登出失败"), nil)
			return
		} else {
			response.JSON(ctx, errors.Wrap(err, ecode.Success, "登出成功"), nil)
		}
	}
}

// 刷新token
func (handler *UserHandler) UserRefresh() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := handler.service.UserRefresh(ctx)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.UserLoginErr, "Token刷新失败"), nil)
		} else {
			_ = handler.service.UserLogSave(ctx, "refresh_token", "success")
			response.JSON(ctx, errors.Wrap(err, ecode.Success, "Token刷新成功"), res)
		}
	}
}

// 检查登陆状态
func (handler *UserHandler) UserAuthStatus() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := handler.service.UserAuthStatus(ctx)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.UserLoginErr, "无效的token"), res)
		} else {
			response.JSON(ctx, errors.Wrap(err, ecode.Success, ""), res)
		}
	}
}

// @Summary		验证邮箱是否可用
// @title			Chat api
// @version		1.0
// @description	验证邮箱是否可用
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.UserVerifyEmailReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse{data=model.UserVerifyEmailRes}
// @Router			/api/v1/auth/checkemail [post]
func (handler *UserHandler) UserVerifyEmail() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserVerifyEmailReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		res, err := handler.service.UserVerifyEmail(ctx, req.Email)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// @Summary		验证用户名是否可用
// @title			Chat api
// @version		1.0
// @description	验证用户名是否可用
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.UserVerifyUsernameReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse{data=model.UserVerifyUsernameRes}
// @Router			/api/v1/auth/checkusername [post]
func (handler *UserHandler) UserVerifyUsername() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserVerifyUsernameReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		res, err := handler.service.UserVerifyUserName(ctx, req.Username)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// @Summary		更改用户昵称
// @title			Swagger API
// @version		1.0
// @description	修改用户昵称
// @Accept			application/json
// @Produce		application/json
// @Param			Authorization	header		string						false	"Bearer 用户令牌"
// @Param			object	body		model.UserUpdateNicknameReq	true	"查询参数"
// @Success		200				{object}	response.ApiResponse{data=model.UserUpdateNicknameRes}
// @Router			/api/v1/user/changenickname [post]
func (handler *UserHandler) UserUpdateNickname() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserUpdateNicknameReq
		userId := ctx.GetInt64(consts.UserID)
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		res, err := handler.service.UserUpdateNickName(ctx, userId, req.Nickname)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// @Summary		获取用户账单
// @title			Swagger API
// @version		1.0
// @description	获取用户获得积分的账单
// @Accept			application/json
// @Produce		application/json
// @Param			Authorization	header		string					false	"Bearer 用户令牌"
// @Param			object	body		model.UserBillGetReq	true	"查询参数"
// @Success		200				{object}	response.ApiResponse
// @Router			/api/v1/user/bill [post]
func (handler *UserHandler) UserBillGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserBillGetReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		res, err := handler.service.UserBillGet(ctx, req)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			encryptData, err := handler.ds.UserDeviceEncrypt(ctx, res)
			if err != nil {
				response.JSON(ctx, errors.Wrap(err, ecode.ValidateErr, "Encrypt data error."), nil)
				return
			}
			response.JSONEncrypted(ctx, nil, encryptData)
		}
	}
}

// @Summary		用户激活
// @title			Chat api
// @version		1.0
// @description	用户激活，通过邮箱注册的，需要激活用户，验证邮箱
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.UserActiveReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse
// @Router			/api/v1/auth/active [post]
func (handler *UserHandler) UserActive() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserActiveReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		if !handler.service.UserTempCodeVerify(ctx, req.ActiveCode) {
			if handler.service.UserActiveVerify(ctx) {
				response.JSON(ctx, nil, nil)
				return
			}
			response.JSON(ctx, errors.WithCode(ecode.ActiveErr, "用户激活失败"), nil)
			return
		}
		if err := handler.service.UserActiveChange(ctx); err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.ActiveErr, "用户激活失败"), nil)
			return
		}
		response.JSON(ctx, nil, nil)
	}
}

// @Summary		修改用户密码
// @title			Swagger API
// @version		1.0
// @description	根据旧密码修改为新的密码
// @Accept			application/json
// @Produce		application/json
// @Param			Authorization	header		string						false	"Bearer 用户令牌"
// @Param			object	body		model.UserPasswordModifyReq	true	"查询参数"
// @Success		200				{object}	response.ApiResponse
// @Router			/api/v1/user/updatepassword [post]
func (handler *UserHandler) UserPasswordModify() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserPasswordModifyReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		if !handler.service.UserPasswordVerify(ctx, req.OldPassword) {
			response.JSON(ctx, errors.WithCode(ecode.PasswordErr, "密码错误"), nil)
			return
		}
		if err := handler.service.UserPasswordModify(ctx, req.NewPassword); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.Unknown, "密码更新失败"), nil)
			return
		}
		response.JSON(ctx, nil, nil)
	}
}

// @Summary		用户密码找回
// @title			Chat api
// @version		1.0
// @description	用户密码找回
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.UserForgetReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse
// @Router			/api/v1/auth/forget [post]
func (handler *UserHandler) UserPasswordForget() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserForgetReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		iscode := handler.service.CaptchaVerify(ctx, req.Captcha)
		if !iscode {
			response.JSON(ctx, errors.WithCode(ecode.CaptchaErr, "验证码错误"), nil)
			return
		}
		isemail, err := handler.service.UserVerifyEmail(ctx, req.Email)
		if err != nil || isemail.IsValid {
			response.JSON(ctx, errors.WithCode(ecode.Unknown, "邮箱不存在"), nil)
			return
		}
		if err := handler.service.UserPasswordForget(ctx); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.Unknown, "密码重置邮件发送失败"), nil)
			return
		}
		response.JSON(ctx, nil, nil)
	}
}

// @Summary		重置用户密码
// @title			Chat api
// @version		1.0
// @description	重置用户密码
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.UserPasswordResetReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse
// @Router			/api/v1/auth/resetpassword [post]
func (handler *UserHandler) UserPasswordReset() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.UserPasswordResetReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		if !handler.service.UserTempCodeVerify(ctx, req.TempCode) {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "验证码错误"), nil)
			return
		}
		if err := handler.service.UserPasswordModify(ctx, req.NewPassword); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.Unknown, "密码更新失败"), nil)
			return
		}
		response.JSON(ctx, nil, nil)
	}
}

// @Summary		获取当前用户的邀请链接
// @title			Swagger API
// @version		1.0
// @description	获取当前用户的邀请链接
// @Produce		application/json
// @Param			Authorization	header		string				false	"Bearer 用户令牌"
// @Param			object	body		true	"查询参数"
// @Success		200				{object}	response.ApiResponse{data=model.UserInviteLinkRes}
// @Router			/api/v1/user/invitelink [post]
func (handler *UserHandler) UserInviteLinkGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := handler.service.UserInviteLinkGet(ctx)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "邀请链接获取失败"), nil)
			return
		}
		response.JSON(ctx, nil, res)
	}
}

// @Summary		获取验证码
// @title			Swagger API
// @version		1.0
// @description	获取验证码用来进行安全验证
// @Produce		json
// @Success		200	{object}	response.ApiResponse{data=model.CaptchaRes}	"desc"
// @Router			/api/v1/auth/captcha [post]
func (handler *UserHandler) CaptchaGen() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := handler.service.CaptchaGen(ctx)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "验证码获取失败"), nil)
			return
		}
		response.JSON(ctx, nil, res)
	}
}

// @Summary		inapps 内购购买验证
// @title			Swagger API
// @version		1.0
// @description	inapps 内购购买
// @Produce		json
//
//	@Param Language-Id header string true "客户端语言"
//	@Param T-App-Id header string true "客户端包名"
//	@Param Platform-Type header string true "客户端平台：iOS、android、web"
//
// @Param			object	body		model.AppleInAppsPayReq	true	"查询参数"
// @Success		200	{object}	response.ApiResponse	"desc"
// @Router			/api/v1/product/payment/inapps [post]
func (handler *UserHandler) UserInAppPay() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		//var req model.AppleInAppsPayV2Req
		//if err := ctx.ShouldBindJSON(&req); err != nil {
		//	response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
		//	return
		//}
		//err := handler.service.UserInAppPayV2(ctx, req)
		//if err != nil {
		//	response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
		//	return
		//}
		//response.JSON(ctx, nil, nil)
	}
}

// @Summary		上报设备devicetoken
// @title		chater api
// @version		1.0
// @description	每次打开app的时上报设备信息
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.DeviceReportReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse
// @Router			/api/v1/device/report [post]
func (handler *UserHandler) UserReportDevice() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var sign struct {
			D string `json:"d"`
		}

		if err := ctx.ShouldBindJSON(&sign); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		plainBytes, err := security.
			NewRsa("", "deploy/rsa_private.pem").
			DecryptBlockString(sign.D)
		if err != nil {
			return
		}

		var payload model.DeviceReportReq
		err = json.Unmarshal(plainBytes, &payload)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		_, err = uuid.Parse(payload.UUID)
		if err != nil {
			// 解析uuid，必须是UUID
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		payload.UUID = strings.ToLower(payload.UUID)
		err = handler.ds.UserDeviceUpdate(ctx, payload)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
			return
		}
		response.JSON(ctx, nil, nil)
	}
}

// @Summary		上报设备devicetoken
// @title		chater api
// @version		1.0
// @description	每次打开app的时候获取 device_token 并且和上一次对比，如果发生了变化，则需要 调用 /report 上报新的 device_token ，完成 UUID 和 device_token 配对的更新
// @Accept			application/json
// @Produce		application/json
// @Param			object	body		model.DeviceTokenReportReq	true	"查询参数"
// @Success		200		{object}	response.ApiResponse
// @Router			/api/v1/devuce/report/devicetoken [post]
func (uh *UserHandler) UserPushReport() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.ChaChaPolyEncryptData
		// 绑定JSON请求
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		payloadData, err := uh.ds.UserDeviceDecrypt(ctx, req)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "Request parameter decryption failed"), nil)
			return
		}

		var payload model.DeviceTokenReportReq
		err = json.Unmarshal(payloadData, &payload)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		if payload.Platform != consts.PlatformIOS &&
			payload.Platform != consts.PlatformAndroid {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "Platform 不正确"), nil)
			return
		}
		err = uh.ds.UserDeviceTokenUpdate(ctx, payload)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.Unknown, err.Error()), nil)
			return
		}
		response.JSON(ctx, nil, nil)
	}
}

// @Summary		获取所有用户的device token
// @title		chater api
// @version		1.0
// @description	获取所有用户的device token
// @Accept			application/json
// @Produce		application/json
// @Success		200		{object}	response.ApiResponse{Data=model.DeviceTokenListRes}
// @Router			/api/v1/admin/devicetoken/list [post]
func (uh *UserHandler) UserDeviceTokenList() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := uh.ds.UserGetDeviceTokenList1(ctx)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
			return
		}

		response.JSON(ctx, nil, res)
	}
}

// @Summary		获取用户当前用于交易的资金概况和风控配置
// @title		chater api
// @version		1.0
// @description	以供前端进行即时计算和风险提示
// @Accept			application/json
// @Produce		application/json
// @Success		200		{object}	response.ApiResponse{Data=model.DeviceTokenListRes}
// @Router			/api/v1/user/assets [get]
func (uh *UserHandler) UserGetAssets() gin.HandlerFunc {
	return func(ctx *gin.Context) {

	}
}
