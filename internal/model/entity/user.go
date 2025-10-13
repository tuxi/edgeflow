package entity

import (
	"edgeflow/utils"
	"gorm.io/plugin/soft_delete"
	"time"
)

type User struct {
	Id              int64                 `gorm:"column:id;primary_key;" json:"id"`
	Username        string                `gorm:"column:username;not null;unique" json:"username"` // unique 用户名唯一且不能为空
	Nickname        string                `gorm:"column:nickname" json:"nickname"`
	Email           string                `gorm:"column:email;unique" json:"email"` // unique 邮箱号号唯一
	Phone           string                `gorm:"column:phone;unique" json:"phone"` // unique 手机号唯一
	AvatarUrl       string                `gorm:"column:avatar_url" json:"avatar_url"`
	Password        string                `gorm:"column:password" json:"password"`
	RegisteredIp    string                `gorm:"column:registered_ip" json:"registered_ip"`
	IsActive        bool                  `gorm:"column:is_active" json:"is_active"`
	Balance         float64               `gorm:"column:balance" json:"balance"`
	Role            int                   `gorm:"column:role;default 1" json:"role"`                             // 角色
	IsAnonymous     bool                  `gorm:"column:is_anonymous" json:"is_anonymous"`                       // 是否为匿名用户
	IsAdministrator bool                  `gorm:"column:is_administrator;default:false" json:"is_administrator"` // 是否为管理员
	CreatedAt       utils.JsonTime        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       utils.JsonTime        `gorm:"column:deleted_at" json:"deleted_at" `
	IsDel           soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
	Bills           []Bill                `gorm:"foreignKey:user_id;references:id"`
	UserLogs        []UserLog             `gorm:"foreignKey:user_id;references:id"`
}

func (User) TableName() string {
	return "user"
}

// 角色订阅
type SubscribeProduct struct {
	Id         int64                 `gorm:"column:id;primary_key;" json:"id"`
	Role       int                   `gorm:"column:role;" json:"role"`
	Price      float64               `gorm:"column:price;" json:"price"`
	Identifier string                `gorm:"identifier;not null;unique" json:"identifier"`
	Days       int                   `gorm:"column:days;" json:"days"` // 订阅天数
	CreatedAt  utils.JsonTime        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt  utils.JsonTime        `gorm:"column:deleted_at" json:"deleted_at"`
	IsDel      soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
}

func (SubscribeProduct) TableName() string {
	return "subscribeproduct"
}

// 关联订阅和用户之间的关系
// User表只需要关心用户的基本信息和身份，Subscribe表只需要关心订阅的细节，而UserSubscriptions表则负责管理两者之间的关系。这样的设计更符合单一职责原则，有助于代码的维护和扩展。
type UserSubscriptions struct {
	Id                 int64                 `gorm:"column:id;primary_key;" json:"id"`
	UserId             int64                 `gorm:"column:user_id" json:"user_id"`
	SubscribeProductID int64                 `gorm:"column:subscribe_product_id" json:"subscribe_product_id"`
	User               User                  `gorm:"foreignKey:user_id"`
	SubscribeProduct   SubscribeProduct      `gorm:"foreignKey:subscribe_product_id"`
	CreatedAt          utils.JsonTime        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt          utils.JsonTime        `gorm:"column:deleted_at" json:"deleted_at"`
	StartedAt          utils.JsonTime        `gorm:"column:started_at" json:"started_at"`
	ExpiredAt          utils.JsonTime        `gorm:"column:expired_at" json:"expired_at"`
	IsDel              soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
}

func (UserSubscriptions) TableName() string {
	return "usersubscriptions"
}

func (s *UserSubscriptions) IsExpired() bool {
	// 其他订阅过期后，回归标准用户
	expiredAt := time.Time(s.ExpiredAt)
	if expiredAt.IsZero() {
		return false
	}
	now := time.Now()
	return now.After(expiredAt)
}
