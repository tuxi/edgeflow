package model

import (
	"edgeflow/internal/model/entity"
	"edgeflow/utils"
	"fmt"
	"gorm.io/datatypes"
)

// 用户登陆发起请求的参数
type UserLoginReq struct {
	Username string `json:"username" validate:"required" label:"用户名"`
	Password string `json:"password" validate:"required" label:"密码"`
	Captcha  string `json:"captcha" validate:"required" label:"验证码"`
}

// 用户登陆成功响应的结构体
type UserLoginRes struct {
	Token           string `json:"token"`
	Timeout         int    `json:"timeout"`
	IsAnonymous     bool   `json:"is_anonymous"`     // 是否为匿名访问
	IsAdministrator bool   `json:"is_administrator"` // 是否为管理员
	Role            int    `json:"role"`             // 角色
	IsNew           bool   `json:"is_new"`
	Key             string `json:"key"` // 服务端生成的公钥，客户端用它来解密的
}

// 用户的token状态
type UserAuthStatusRes struct {
	// 是否无效
	IsInvalid bool `json:"is_invalid"`
}

// 用户注册的参数
type UserRegisterReq struct {
	Username   string `json:"username" validate:"required,username"  label:"用户名"`
	Password   string `json:"password" validate:"required"  label:"密码"`
	Email      string `json:"email" validate:"required"  label:"邮箱地址"`
	Captcha    string `json:"captcha" validate:"required"  label:"验证码"`
	InviteCode string `json:"invite_code" label:"邀请码"`
}

type UserRegisterRes struct {
	IsSuccess bool `json:"is_success"`
}

type UserVerifyUsernameReq struct {
	Username string `json:"username" validate:"required" label:"用户名"`
}

type UserVerifyUsernameRes struct {
	IsValid bool `json:"is_valid"`
}

type UserVerifyEmailReq struct {
	Email string `json:"email" validate:"required" label:"邮箱"`
}

type UserVerifyEmailRes struct {
	IsValid bool `json:"is_valid"`
}

type UsergetAvatarRes struct {
	AvatarUrl string `json:"avatar_url"`
}

type UserGetInfoRes struct {
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	RoleName  string `json:"role_name"`
	Role      int    `json:"role"`
	UserId    int64  `json:"user_id"`
	AvatarUrl string `json:"avatarUrl"`
}

type UserInfo struct {
	UserId          int64  `gorm:"column:id" json:"user_id"`
	Username        string `gorm:"column:username" json:"username"`
	Nickname        string `gorm:"column:nickname" json:"nickname"`
	Password        string `gorm:"column:password" json:"password"`
	Email           string `gorm:"column:email" json:"email"`
	Phone           string `gorm:"column:phone" json:"phone"`
	Role            int    `gorm:"column:role" json:"role"`
	IsActive        bool   `gorm:"column:is_active" json:"is_active"`
	IsAnonymous     bool   `gorm:"column:is_anonymous;default:false" json:"is_anonymous"`
	AvatarUrl       string `json:"avatarUrl"`
	IsAdministrator bool   `gorm:"column:is_administrator;default:false" json:"is_administrator"`
}

type UserAvatarRes struct {
	// 头像的base64
	Avatar    string `json:"avatar"`
	AvatarURI string `json:"avatar_uri"`
}

type UserActiveReq struct {
	ActiveCode string `form:"active_code" json:"active_code" validate:"required"`
}

type UserUpdateNicknameReq struct {
	Nickname string `json:"nickname" validate:"required"  label:"用户昵称"`
}

type UserUpdateNicknameRes struct {
	IsChanged bool `json:"is_changed"`
}
type UserBalance struct {
	Balance float64 `json:"balance"`
}

type UserPasswordModifyReq struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

type UserPasswordResetReq struct {
	TempCode    string `json:"temp_code" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

type UserForgetReq struct {
	Email   string `json:"email"`
	Captcha string `json:"captcha" validate:"required"  label:"验证码"`
}

type UserInviteLinkRes struct {
	InviteLink   string  `json:"invite_link"`
	InviteNumber int     `json:"invite_number"`
	InviteReward float64 `json:"invite_reward"`
}

type CaptchaRes struct {
	Image string `json:"image"`
}

type UserBillGetReq struct {
	Page     int `form:"page" json:"page" validate:"required"`           // 页面码
	PageSize int `form:"page_size" json:"page_size" validate:"required"` // 每页的数量
	EndTime  int `form:"end_time" json:"end_time"`                       // 查询的截止时间（毫秒时间戳），如果为空则为当前时间
	Days     int `form:"days" json:"days"`                               // 查询天数，如果为0，则查询到达EndTime时间一年的记录
}

type UserBillRes struct {
	BillId      int64          `gorm:"column:id" json:"bill_id"`              // 账单id
	CreatedAt   utils.JsonTime `gorm:"column:created_at" json:"change_time"`  // 创建时间
	CostChange  float64        `gorm:"column:cost_change" json:"change"`      // 发生改变的金额
	Balance     float64        `gorm:"column:balance" json:"balance"`         // 余额
	CostComment string         `gorm:"column:cost_comment" json:"comment"`    // 描述发生来源
	Extras      datatypes.JSON `gorm:"column:extras;type:json" json:"extras"` // 第三方订单信息，比如苹果内购的订单及校验信息
	BillType    int            `gorm:"column:bill_type" json:"bill_type"`     // 账单类型
}

type UserBillListRes struct {
	BillList []UserBillRes  `json:"bill_list"` // 账单列表
	Plan     UserPlanGetRes `json:"plan"`      // 计划
}

type UserLogDeviceInfo struct {
	// 屏幕高度
	ScreenHeight int64 `gorm:"screen_height" json:"screen_height"`
	// 屏幕宽度
	ScreenWidth int64 `gorm:"screen_width" json:"screen_width"`
	// 系统名称
	Os string `gorm:"os" json:"os"`
	// 系统版本号
	OsVersion string `gorm:"os_version" json:"os_version"`
	// app编译版本号
	AppBuildNumber string `gorm:"app_build_number" json:"app_build_number"`
	// App 版本号
	AppVersion string `gorm:"app_version" json:"app_version"`
	// 平台 appstore或者小米商店
	AppPlatform string `gorm:"app_platform" json:"app_platform"`
	// app包名
	AppPackageId string `gorm:"app_package_id" json:"app_package_id"`
	// 设备品牌
	DeviceBrand string `gorm:"device_brand" json:"device_brand"`
	// 设备id
	DeviceId string `gorm:"device_id" json:"device_id"`
	// 设备型号
	DeviceModel string `gorm:"device_model" json:"device_model"`
	Radio       string `gorm:"radio" json:"radio"`
	// 运营商
	Carrier string `gorm:"carrier" json:"carrier"`
	// 是否是wifi
	IsWifi bool `gorm:"is_wifi" json:"is_wifi"`
	// 使用的代理
	Proxy string `gorm:"proxy" json:"proxy"`
	// 设备型号描述
	DeviceModelDesc string `gorm:"device_model_desc" json:"device_model_desc"`
	// 使用的语言
	LanguageId string `gorm:"language_id" json:"language_id"`
}

type UserLogsGetReq struct {
	Limit int    `json:"limit" form:"limit" uri:"limit"`
	Page  int    `json:"page" form:"page" uri:"page"`
	Sort  string `json:"sort" form:"sort" uri:"sort"`
}

type UserLogsGetRes struct {
	UserLogs []entity.UserLog `json:"user_logs"`
}

// 余额是字数而非钱币，用户充值后，被转换为字数存储
type UserBalanceGetRes struct {
	Balance    float64 `json:"balance"`     // 余额
	BalanceStr string  `json:"balance_str"` // 余额，显示的字符串
	Tokens     int64   `json:"tokens"`      // 当前余额可用于对话的字数
}

func NewUserBalanceGetRes(balance float64) UserBalanceGetRes {
	return UserBalanceGetRes{
		Balance:    balance,
		BalanceStr: fmt.Sprintf("%.5f", balance),
	}
}

type UserBillGetByReceiptReq struct {
	UserId  int64  `json:"user_id" validate:"required"`
	Receipt string `json:"receipt" validate:"required"`
}

type UserSubscriptionsRes struct {
	Role      int            `json:"role"`
	StartedAt utils.JsonTime `json:"started_at"`
	ExpiredAt utils.JsonTime `json:"expired_at"`
	IsExpired bool           `json:"is_expired"`
}

// 余额是字数而非钱币，用户充值后，被转换为字数存储
type UserPlanGetRes struct {
	Balance      UserBalanceGetRes    `json:"balance"`
	Subscription UserSubscriptionsRes `json:"subscription"`
}

type UserAssetsGetRes struct {
	UserID                 string  `json:"user_id"`                    //"uuid-12345"	确认身份。
	AvailableBalanceUSDT   string  `json:"available_balance_usdt"`     //	Decimal	10000.00	用户在连接交易所的可用余额，用于计算投入金额。
	ExchangeStatus         string  `json:"exchange_status"`            // "CONNECTED"	交易所 API 连接状态，若为 DISCONNECTED，下单按钮应置灰。
	MaxRiskPerTradePercent float32 `json:"max_risk_per_trade_percent"` //	Decimal	1.0	核心风控参数。 用户设定的单笔最大亏损占总账户余额的百分比（如 1%）。
	DefaultLeverage        int     `json:"default_leverage"`           // Integer	5 如果策略在合约市场执行，提供默认杠杆值供参考。
	CopyTradingEnabled     bool    `json:"copy_trading_enabled"`       //	Boolean	true	用户是否已开通 Hyperliquid 跟单服务权限。
	LastFetchTimestamp     int     `json:"last_fetch_timestamp"`       // 	Unix Timestamp	1678886400	资产数据的最后更新时间，体现数据的时效性。
}
