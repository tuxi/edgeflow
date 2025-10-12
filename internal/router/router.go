package router

import (
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/instrument"
	"edgeflow/internal/handler/market"
	"edgeflow/internal/handler/signal"
	"edgeflow/internal/handler/user"
	"edgeflow/internal/middleware"
	"github.com/gin-gonic/gin"
)

type ApiRouter struct {
	coinHandler   *instrument.Handler
	hyperHandler  *hyperliquid.Handler
	mh            *market.MarketHandler
	userHandler   *user.UserHandler
	signalHandler *signal.SignalHandler
}

func NewApiRouter(ch *instrument.Handler, mh *market.MarketHandler, hyperHandler *hyperliquid.Handler, userHandler *user.UserHandler, SignalHandler *signal.SignalHandler) *ApiRouter {
	return &ApiRouter{coinHandler: ch, mh: mh, hyperHandler: hyperHandler, userHandler: userHandler, signalHandler: SignalHandler}
}

func (api *ApiRouter) Load(g *gin.Engine) {

	// auth
	base := g.Group("/api/v1")

	c := base.Group("/instruments", middleware.RequestValidationMiddleware())
	{
		// 获取币种列表
		c.GET("/list", api.coinHandler.CoinsGetList())
		c.GET("/all", api.coinHandler.InstrumentGetAll())
	}

	p := base.Group("/ticker", middleware.RequestValidationMiddleware())
	{
		p.GET("/ws", api.mh.ServeWS) // 通过websocket连接获取价格

	}

	h := base.Group("/hyperliquid", middleware.RequestValidationMiddleware())
	{
		h.GET("/whales/address", api.hyperHandler.WhaleInfoGetByAddress())
		h.POST("/whales/leaderboard", api.hyperHandler.WhaleLeaderboardGet())
		h.POST("/whales/account", api.hyperHandler.WhaleAccountSummaryGet())
		h.GET("/whales/fills-orders", api.hyperHandler.WhaleUserFillOrderHistoryGet())
		h.GET("/whales/open-orders", api.hyperHandler.WhaleUserOpenOrderHistoryGet())
		h.GET("/whales/nonfunding", api.hyperHandler.WhaleUserNonFundingLedgerGet())
		h.GET("/whales/top-positions", api.hyperHandler.TopWhalePositionsGet())
		h.GET("/whales/positions-analyze", api.hyperHandler.TopWhalePositionsAnalyze())

	}

	sg := base.Group("/signal", middleware.RequestValidationMiddleware())
	{
		sg.GET("/list", api.signalHandler.SignalGetList())
		sg.GET("/detail", api.signalHandler.GetSignalDetailByID())
		sg.POST("/execute", api.signalHandler.ExecuteSignal())
	}

	u := base.Group("/user", middleware.AuthToken())
	{
		u.GET("/avatar", api.userHandler.UserGetAvatar())
		u.GET("/info", api.userHandler.UserGetInfo())
		u.GET("/logout", api.userHandler.UserLogout())
		u.POST("/changenickname", api.userHandler.UserUpdateNickname())
		u.POST("/updatepassword", api.userHandler.UserPasswordModify())
		u.GET("/refresh", api.userHandler.UserRefresh())
		u.GET("/status", api.userHandler.UserAuthStatus())
		u.GET("/invitelink", api.userHandler.UserInviteLinkGet())
		u.GET("/bill", api.userHandler.UserBillGet())
		u.GET("/balance", api.userHandler.UserGetBalance())
		u.GET("/plan", api.userHandler.UserGetPlan())
		// 获取用户当前用于交易的资金概况和风控配置，以供前端进行即时计算和风险提示。
		u.GET("/assets", api.userHandler.UserGetBalance())
	}

	auth := base.Group("/auth")
	{
		auth.POST("/login", api.userHandler.UserLogin())
		auth.POST("/register", api.userHandler.UserRegister())
		auth.POST("/checkemail", api.userHandler.UserVerifyEmail())
		auth.POST("/checkusername", api.userHandler.UserVerifyUsername())
		auth.POST("/active", api.userHandler.UserActive())
		auth.POST("/forget", api.userHandler.UserPasswordForget())
		auth.POST("/resetpassword", api.userHandler.UserPasswordReset())
		auth.POST("/captcha", api.userHandler.CaptchaGen())
		// 根据sign获取一个匿名用户的token，没有则创建一个匿名用户
		auth.POST("/anonymous/accessToken", api.userHandler.GetAnonymousAccessToken())
	}

	//base.POST("/webhook", middleware.RequestValidationMiddleware(), api.wh.HandlerWebhook())
}
