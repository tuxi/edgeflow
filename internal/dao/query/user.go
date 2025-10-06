package query

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
)

var _ dao.UserDao = (*userDao)(nil)

type userDao struct {
	ds *gorm.DB
}

func NewUserDao(ds *gorm.DB) *userDao {
	return &userDao{
		ds: ds,
	}
}

func (u *userDao) UserGetByName(ctx context.Context, username string) (entity.User, error) {
	var user entity.User
	err := u.ds.WithContext(ctx).Model(&entity.User{}).Where("username = ?", username).Find(&user).Error
	return user, err
}

func (u *userDao) UserGetById(ctx context.Context, userId int64) (model.UserInfo, error) {
	var user model.UserInfo
	err := u.ds.WithContext(ctx).Model(&entity.User{}).Where("id = ?", userId).Find(&user).Error
	return user, err
}

func (u *userDao) UserCreate(ctx context.Context, user *entity.User) error {
	var existingUser entity.User
	// username唯一出现问题，处理下：
	// 数据库级别的唯一约束不能完全防止竞态条件，也就是当两个请求几乎同时尝试插入相同的用户名时，可能会出现问题。
	if err := u.ds.WithContext(ctx).Where("username = ?", user.Username).First(&existingUser).Error; err != gorm.ErrRecordNotFound {
		return err
	}
	return u.ds.WithContext(ctx).Create(user).Error
}

func (u *userDao) UserVerifyEmail(ctx context.Context, email string) (userId int64, err error) {
	err = u.ds.WithContext(ctx).Model(&entity.User{}).Where("email = ?", email).Select("id").Find(&userId).Error
	return
}

func (u *userDao) UserVerifyUsername(ctx context.Context, username string) (count int64, err error) {
	err = u.ds.WithContext(ctx).Model(&entity.User{}).Where("username = ?", username).Count(&count).Error
	return
}

func (u *userDao) UserUpdateNickName(ctx context.Context, userId int64, nickname string) error {
	err := u.ds.WithContext(ctx).Model(&entity.User{}).Where("id = ?", userId).Update("nickname", nickname).Error
	return err
}

func (u *userDao) UserSubscriptionsCreate(ctx context.Context, userSubscriptions *entity.UserSubscriptions) error {
	// 创建
	err := u.ds.WithContext(ctx).Create(userSubscriptions).Error
	return err
}

func (u *userDao) UserSubscriptionsUpdate(ctx context.Context, userSubscriptions *entity.UserSubscriptions) error {
	err := u.ds.WithContext(ctx).Updates(userSubscriptions).Error
	return err
}

func (u *userDao) UserDelete(ctx context.Context, useriId int64) error {
	return u.ds.WithContext(ctx).Delete(&entity.User{Id: useriId}).Error
}

func (u *userDao) UserUpdate(ctx context.Context, user *entity.User) error {
	return u.ds.WithContext(ctx).Updates(user).Error
}

func (u *userDao) UserSubscriptionsGet(ctx context.Context, userId int64) (userSubscriptions *entity.UserSubscriptions, err error) {
	err = u.ds.WithContext(ctx).Where("user_id = ?", userId).Preload("SubscribeProduct").Find(&userSubscriptions).Error
	return
}

func (u *userDao) UserGetIsAdministrator(ctx context.Context, userId int64) (isAdministrator bool, err error) {
	err = u.ds.WithContext(ctx).Model(entity.User{}).Where("id = ?", userId).Select("is_administrator").Find(&isAdministrator).Error
	return
}

func (u *userDao) UserGetAvatar(ctx context.Context, userId int64) (string, error) {
	var avatar string
	err := u.ds.WithContext(ctx).Model(&entity.User{}).Where("id = ?", userId).Select("avatar_url").Find(&avatar).Error
	return avatar, err
}

func (u *userDao) UserGetBalance(ctx context.Context, userId int64) (float64, error) {
	var balance float64
	err := u.ds.WithContext(ctx).Model(&entity.User{}).Where("id = ?", userId).Select("balance").Find(&balance).Error
	return balance, err
}

func (u *userDao) UserBillCreate(ctx context.Context, bill *entity.Bill) error {
	return u.ds.WithContext(ctx).Create(bill).Error
}

func (u *userDao) UserBillGet(ctx context.Context, userId int64, page, pageSize int, start, end string) ([]model.UserBillRes, error) {
	var bills []model.UserBillRes
	// 分页查询
	offset := (page - 1) * pageSize
	// .Order("id desc")
	err := u.ds.WithContext(ctx).Model(&entity.Bill{}).Where("user_id = ?", userId).Where("created_at BETWEEN ? AND ?", start, end).Limit(pageSize).Offset(offset).Order("created_at desc").Find(&bills).Error
	return bills, err
}

func (u *userDao) UserInviteGen(ctx context.Context, invite *entity.Invite) error {
	return u.ds.WithContext(ctx).Create(invite).Error
}

func (u *userDao) UserInviteGetByUser(ctx context.Context, userId int64) (entity.Invite, error) {
	var inviteCode entity.Invite
	err := u.ds.WithContext(ctx).Where("user_id = ?", userId).Find(&inviteCode).Error
	return inviteCode, err
}

func (u *userDao) UserInviteGetByCode(ctx context.Context, code string) (entity.Invite, error) {
	var inviteCode entity.Invite
	err := u.ds.WithContext(ctx).Where("invite_code = ?", code).Find(&inviteCode).Error
	return inviteCode, err
}

func (u *userDao) UserInviteUpdate(ctx context.Context, invite *entity.Invite) error {
	return u.ds.WithContext(ctx).Updates(invite).Error
}

func (u *userDao) AnonymousUserGetByAnonymousUid(ctx context.Context, anonymousUid string) (entity.User, error) {
	var user entity.User
	err := u.ds.WithContext(ctx).Where("username = ?", anonymousUid).Where("is_anonymous = ?", true).First(&user).Error
	return user, err
}

func (u *userDao) UserAddLog(ctx context.Context, log *entity.UserLog) error {
	return u.ds.WithContext(ctx).Create(log).Error
}

func (u *userDao) UserLogsGet(ctx context.Context, req model.UserLogsGetReq) ([]entity.UserLog, error) {
	var userLogs []entity.UserLog
	// 分页查询
	offset := (req.Page - 1) * req.Limit
	err := u.ds.WithContext(ctx).Model(&entity.UserLog{}).Limit(req.Limit).Offset(offset).Order(req.Sort).Find(&userLogs).Error
	return userLogs, err
}

func (u *userDao) UserBillGetTransactionId(ctx context.Context, userId int64, transactionId string) (res entity.Bill, err error) {
	err = u.ds.WithContext(ctx).Model(entity.Bill{}).Where(entity.Bill{UserId: userId, TransactionId: transactionId}).First(&res).Error
	return
}

func (u *userDao) SubscribeProductGet(ctx context.Context, identifier string) (subscribe entity.SubscribeProduct, err error) {
	err = u.ds.WithContext(ctx).Where("identifier = ?", identifier).First(&subscribe).Error
	return
}
