package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type UserDao interface {
	// 根据用户名获取user实体
	UserGetByName(ctx context.Context, username string) (entity.User, error)
	// 根据用户名称获取用户
	UserGetById(ctx context.Context, userId int64) (model.UserInfo, error)
	// 创建用户
	UserCreate(ctx context.Context, user *entity.User) error
	// 用户邮箱验证
	UserVerifyEmail(ctx context.Context, email string) (userId int64, err error)
	// 用户名称校验
	UserVerifyUsername(ctx context.Context, username string) (count int64, err error)
	// 用户昵称更新
	UserUpdateNickName(ctx context.Context, userId int64, nickname string) error
	// 创建订阅
	UserSubscriptionsCreate(ctx context.Context, subscribe *entity.UserSubscriptions) error
	// 更新用户角色
	UserSubscriptionsUpdate(ctx context.Context, userSubscriptions *entity.UserSubscriptions) error
	// 删除用户
	UserDelete(ctx context.Context, useriId int64) error
	// 更新用户
	UserUpdate(ctx context.Context, user *entity.User) error
	// 获取用户角色
	UserSubscriptionsGet(ctx context.Context, userId int64) (role *entity.UserSubscriptions, err error)
	// 获取是否是管理员用户
	UserGetIsAdministrator(ctx context.Context, userId int64) (isAdministrator bool, err error)
	// 获取用户头像地址
	UserGetAvatar(ctx context.Context, userId int64) (string, error)
	// 获取用户余额
	UserGetBalance(ctx context.Context, userId int64) (float64, error)
	// 用户账单创建
	UserBillCreate(ctx context.Context, bill *entity.Bill) error
	// 用户账单获取
	UserBillGet(ctx context.Context, userId int64, page, pageSize int, start, end string) ([]model.UserBillRes, error)

	// 生成邀请码
	UserInviteGen(ctx context.Context, invite *entity.Invite) error
	// 根据用户获取邀请码
	UserInviteGetByUser(ctx context.Context, userId int64) (entity.Invite, error)
	// 根据code获取用户邀请码
	UserInviteGetByCode(ctx context.Context, code string) (entity.Invite, error)
	// 用户邀请码更新
	UserInviteUpdate(ctx context.Context, invite *entity.Invite) error

	// 根据设备id获取匿名用户
	AnonymousUserGetByAnonymousUid(ctx context.Context, anonymousUid string) (entity.User, error)
	// 添加日志
	UserAddLog(ctx context.Context, log *entity.UserLog) error
	// 查询日志
	UserLogsGet(ctx context.Context, req model.UserLogsGetReq) ([]entity.UserLog, error)
	// 根据回执单查询账单记录
	UserBillGetTransactionId(ctx context.Context, userId int64, transactionId string) (res entity.Bill, err error)
	// 根据标识符获取订阅信息
	SubscribeProductGet(ctx context.Context, identifier string) (subscribe entity.SubscribeProduct, err error)
}
