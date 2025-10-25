package consts

import "time"

const (
	// RequestId 请求id名称
	RequestId    = "request_id"
	UserID       = "user_id"
	CostTokenCtx = "cost_token_ctx"
	JWTTokenCtx  = "token_ctx"

	// 请求的SecretKey
	RequestSecretKey = "5b2b13c8a93f1646ee1aaebd8e08a3b5"
	CDKEYBASE        = "E8S2DZX9WYLTN6BQA7CP5IK3MJUFR4HV"
	InviteBase       = "E8uvS2pqDZXbcde9WYfiLTNrs6BxQA7CPmn5IyzK3MwJUktFghR4HVaj"

	UserDeviceInfoPrefix   = "User_Device_info:2"
	UserSubscriptionPrefix = "User_Subscription_list:"
	RegisterAnonymousSlat  = "objc.com.anonymous"
	UserInfoPrefix         = "User_Info_list_2:"
	UserBalancePrefix      = "User_Balance_list:"
	UserAvatarPrefix       = "User_Avatar_url_list:"
	UserInviteLinkPrefix   = "User_Invite_Link_list:"
	CaptchaPrefix          = "Captchat_list:"
	UserInvitePrefix       = "User_Invite_relation_list:"

	// 默认redis过期时间
	RedisExrDefault = time.Hour * 24 * 5

	CBCKEY = "EBCOELCAEBCAEBCS"

	// 邀请奖励3元
	InviteReward = 3
	// 注册奖励8元
	RegisterReward = 6
)

const (
	LanguageId    = "T-Language-Id"
	PlatformType  = "T-Platform-Type"
	ClientId      = "T-App-Id"
	ClientVersion = "T-App-Version"
	DeviceId      = "T-D-Id"
	Timestamp     = "T-Timestamp"
	Signature     = "T-Signature"

	DateLayout   = "2006-01-02"
	TimeLayout   = "2006-01-02 15:04:05"
	TimeLayoutMs = "2006-01-02 15:04:05.000"
)

const (
	WhaleAccountSummaryKey             = "WhaleAccountSummary"
	UserOpenOrderKey                   = "UserOpenOrder"
	UserFillOrderKey                   = "UserFillOrder"
	UserNonFundingLedger               = "UserNonFundingLedger"
	WhalePositionsTop                  = "whale:positions:top"
	HyperWhaleLeaderBoardInfoByAddress = "whale:boardInfo:address"
	WhalePositionsAnalyze              = "WhalePositionsAnalyze"

	// Redis密钥，用于存储获胜率累积统计数据（哈希）
	WhaleWinRateStatsKey = "whale:winrate:stats"
	// 用于存储最终胜率排序集（ZSET）的Redis密钥
	WhaleWinRateZSetKey = "whale:winrate:ranking"
)

// 账单类型
type BillType int

const (
	// 积分相关
	BillTypePointRechargeByPurchase BillType = iota + 1 // 通过iOS内购购买的积分
	BillTypePointRechargeByCDKey                        // 通过cdkey兑换的积分
	BillTypePointConsumedByToken                        // 聊天中消耗的积分
	BillTypePointConsumedByImage                        // 生成图片消耗的积分
	BillTypePointRewardByRegister                       // 注册奖励赠送的积分
	BillTypePointRewardByInvite                         // 邀请别人赠送的积分
	BillTypePointRewardByInvited                        // 被别人邀请赠送的积分
	// 会员相关
	BillTypeMemberRecharge // 通过iOS内购购买的会员服务
	BillTypeMemberExpiry   // 会员服务到期
	// 退款相关
	BillTypePointRefund  // 积分退款
	BillTypeMemberRefund // 会员退款
)

func (bt BillType) String() string {
	return [...]string{
		"PointRechargeByPurchase",
		"PointRechargeByCDKey",
		"PointConsumedByChat",
		"PointConsumedByImage",
		"MemberRecharge",
		"MemberExpiry",
		"PointRefund",
		"MemberRefund",
	}[bt]
}

const (
	StandardUser = iota + 1
	PlusMember
	Enterprise = 301
)

var RoleToString = map[int]string{
	StandardUser: "Standard", // 标准用户
	PlusMember:   "Plus",     // Plus 订阅用户
	Enterprise:   "企业订阅",     // 企业订阅
}

const (
	PlatformIOS     = "iOS"
	PlatformAndroid = "android"
	PlatformWeb     = "web"
)
