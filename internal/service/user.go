package service

import (
	"context"
	"edgeflow/conf"
	"edgeflow/internal/consts"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/active"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/jwt"
	"edgeflow/pkg/logger"
	"edgeflow/pkg/mail"
	"edgeflow/pkg/verification"
	"edgeflow/utils"
	"edgeflow/utils/security"
	"edgeflow/utils/uuid"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-redis/redis/v8"
	uuid2 "github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

type UserService interface {
	UserRegister(ctx *gin.Context, req model.UserRegisterReq) (res model.UserRegisterRes, err error)
	UserDelete(ctx *gin.Context) error
	UserLogin(ctx *gin.Context, username, password string) (res model.UserLoginRes, err error)
	UserLogout(ctx context.Context, tokenstr string) error
	UserRefresh(ctx *gin.Context) (res model.UserLoginRes, err error)
	UserAuthStatus(ctx *gin.Context) (res model.UserAuthStatusRes, err error)
	GetAnonymousAccessToken(ctx *gin.Context, req model.GetAnonymousAccessTokenReq) (res model.UserLoginRes, isNew bool, err error)

	UserGetInfo(ctx context.Context, userId int64) (res model.UserGetInfoRes, err error)
	UserGetAvatar(ctx *gin.Context) (res model.UserAvatarRes, err error)
	UserBillGet(ctx *gin.Context, req model.UserBillGetReq) (res model.UserBillListRes, err error)
	UserBillGetTransactionId(ctx *gin.Context, transactionId string) (res entity.Bill, err error)

	UserGetPlan(ctx context.Context, userId int64) (res model.UserPlanGetRes, err error)
	UserGetSubscription(ctx context.Context, userId int64) (res model.UserSubscriptionsRes, err error)
	UserGetBalance(ctx context.Context, userId int64) (res model.UserBalanceGetRes, err error)
	UserBalanceChange(ctx context.Context, userId int64, billType consts.BillType, oldbalance, amount float64, comment string, cdKeyId *int64, order *model.UserBillExtras, origBill *entity.Bill) (err error)

	UserVerifyEmail(ctx *gin.Context, email string) (res model.UserVerifyEmailRes, err error)
	UserVerifyUserName(ctx context.Context, username string) (res model.UserVerifyUsernameRes, err error)

	UserUpdateNickName(ctx context.Context, userId int64, nickname string) (res model.UserUpdateNicknameRes, err error)
	UserPasswordVerify(ctx *gin.Context, password string) (isValid bool)
	UserPasswordModify(ctx *gin.Context, password string) (err error)
	UserPasswordForget(ctx *gin.Context) (err error)
	UserSubscribeRole(ctx context.Context, userId int64, role int, identifier string) (uSubscribe *entity.UserSubscriptions, err error)

	// 发送用户激活的通知
	UserActiveGen(ctx *gin.Context) (err error)
	// 修改用户激活状态为成功
	UserActiveChange(ctx *gin.Context) (err error)
	UserActiveVerify(ctx *gin.Context) bool
	// 生成激活码
	UserTempCodeVerify(ctx *gin.Context, tempcode string) (Isvalid bool)
	UserTempCodeGen(ctx *gin.Context) (tempcode string, email string, nickname string, err error)

	UserInviteLinkGet(ctx *gin.Context) (res model.UserInviteLinkRes, err error)
	UserInviteGen(ctx *gin.Context) (string, error)
	UserInviteReward(ctx *gin.Context)
	UserInviteVerify(ctx *gin.Context, code string)

	CaptchaGen(ctx *gin.Context) (res model.CaptchaRes, err error)
	CaptchaVerify(ctx *gin.Context, code string) bool

	UserLogSave(ctx *gin.Context, operation, business string) error
}

// userService 实现UserService接口
type userService struct {
	ud   dao.UserDao
	dd   dao.DeviceDao
	iSrv uuid.SnowNode
	rc   *redis.Client
	ds   DeviceService
}

func NewUserService(ud dao.UserDao, dd dao.DeviceDao, ds DeviceService) *userService {
	return &userService{
		ud:   ud,
		dd:   dd,
		iSrv: *uuid.NewNode(3),
		rc:   cache.GetRedisClient(),
		ds:   ds,
	}
}

func (u *userService) UserRegister(ctx *gin.Context, req model.UserRegisterReq) (res model.UserRegisterRes, err error) {
	user := entity.User{}
	res.IsSuccess = false
	user.Id = u.iSrv.GenSnowID()
	ctx.Set(consts.UserID, user.Id)
	user.Username = req.Username
	user.Nickname = req.Username
	user.RegisteredIp = ctx.ClientIP()
	user.Email = req.Email
	//user.Role = consts.StandardUser
	user.Balance = 0
	user.IsActive = false
	user.IsAnonymous = false
	if err != nil {
		return res, err
	}

	// 由于用postman不方便加密密码，这里模拟加密，用于测试
	//password := security.PasswordEncrypt(req.Password, consts.CBCKEY)

	// 由于前端传输过来的密码是加密过的，所以需要先解密
	password := security.PasswordDecryption(req.Password, consts.CBCKEY)
	// 再加密密码存储
	user.Password, err = security.Encrypt(password)
	if err != nil {
		return res, err
	}
	err = u.ud.UserCreate(ctx, &user)
	if err != nil {
		return res, err
	}
	res.IsSuccess = true
	ctx.Set(consts.UserID, user.Id)
	return
}

func (u *userService) UserDelete(ctx *gin.Context) error {
	userId := ctx.GetInt64(consts.UserID)
	return u.ud.UserDelete(ctx, userId)
}

func (u *userService) UserGetSubscription(ctx context.Context, userId int64) (res model.UserSubscriptionsRes, err error) {
	rdsKey := consts.UserSubscriptionPrefix + strconv.FormatInt(userId, 10)
	jsonBytes, err := u.rc.Get(ctx, rdsKey).Bytes()
	var uSubscriptions *entity.UserSubscriptions
	// 先从缓存中查找
	if err == nil {
		err = json.Unmarshal(jsonBytes, &uSubscriptions)
	}
	if err != nil {
		uSubscriptions, err = u.ud.UserSubscriptionsGet(ctx, userId)
		if err == nil && uSubscriptions.Id != 0 {
			jsonBytes, err = json.Marshal(*uSubscriptions)
			if err != nil {
				logger.Errorf("UserSubscriptionsRes序列化失败:%v", err.Error())
				return
			}
			err = u.rc.Set(ctx, rdsKey, jsonBytes, consts.RedisExrDefault).Err()
			if err != nil {
				logger.Errorf("UserSubscriptionsRes存储Cache失败:%v", err.Error())
				return res, nil
			}
		}
	}

	if err != nil {
		return
	}
	res.Role = consts.StandardUser
	if uSubscriptions != nil && uSubscriptions.Id != 0 && uSubscriptions.IsExpired() != true {
		res.Role = uSubscriptions.SubscribeProduct.Role
	}
	res.StartedAt = uSubscriptions.StartedAt
	res.ExpiredAt = uSubscriptions.ExpiredAt
	res.IsExpired = uSubscriptions.IsExpired()
	return res, nil
}

func (u *userService) UserLogin(ctx *gin.Context, username, password string) (res model.UserLoginRes, err error) {
	userInfo, err := u.ud.UserGetByName(ctx, username)
	if err != nil {
		logger.Infof("查询用户失败:%s", err)
		return res, err
	}
	if username != userInfo.Username {
		logger.Infof("用户不存在: %s", username)
		return res, errors.New(fmt.Sprintf("用户不能存在: %s", username))
	}

	if userInfo.IsActive != true {
		err = errors.New("用户未激活")
		return res, err
	}

	//password = security.PasswordEncrypt(password, consts.CBCKEY)

	if !security.ValidatePassword(security.PasswordDecryption(password, consts.CBCKEY), userInfo.Password) {
		err = errors.New("Password error")
		logger.Infof("密码错误：%s", username)
		return res, err
	}

	role, err := u.UserGetSubscription(ctx, userInfo.Id)
	if err != nil {
		return
	}
	r := rand.New(rand.NewSource(time.Now().Unix()))
	num := r.Intn(100)
	settime := conf.AppConfig.Jwt.JwtTtl + int64(num*9)
	timeout := time.Duration(settime) * time.Second
	expireAt := time.Now().Add(timeout)
	claims := jwt.BuildClaims(expireAt, userInfo.Id, role.Role, false, userInfo.IsAdministrator)
	token, err := jwt.GenToken(claims, conf.AppConfig.Jwt.Secret)
	if err != nil {
		logger.Infof("Jwt Token 生成错误：%s", username)
		return res, err
	}
	res.Token = token
	res.Timeout = int(settime) * 1000
	res.Role = claims.RoleId
	res.IsAnonymous = claims.IsAnonymousUser()
	res.IsAdministrator = claims.IsAdministrator()
	ctx.Set(consts.UserID, userInfo.Id)
	return res, err
}

func (u *userService) GetAnonymousAccessToken(ctx *gin.Context, req model.GetAnonymousAccessTokenReq) (res model.UserLoginRes, isNew bool, err error) {
	plainBytes, err := security.
		NewRsa("", "deploy/rsa_private.pem").
		DecryptBlockString(req.Sign)
	if err != nil {
		return
	}
	// 对参数进行校验
	var sign model.AnonymousAccessSign
	err = json.Unmarshal(plainBytes, &sign)
	if err != nil {
		return
	}

	// 校验UUID
	_, err = uuid2.Parse(sign.UUID)
	if err != nil {
		return
	}

	anonymousId := u.makeAnonymousId(sign)
	userInfo, err := u.ud.AnonymousUserGetByAnonymousUid(ctx, anonymousId)
	if err != nil && err == gorm.ErrRecordNotFound {
		// 创建一个匿名用户
		user, err := u.genAnonymousUser(ctx, anonymousId)
		isNew = true
		if err != nil && err == gorm.ErrRecordNotFound {
			logger.Infof("创建匿名用户失败:%s", err)
			return res, isNew, err
		}
		userInfo = *user
		ctx.Set(consts.UserID, user.Id)
		// 自动激活改用户
		err = u.UserActiveChange(ctx)
		if err != nil {
			// 激活失败 删除用户
			_ = u.UserDelete(ctx)
			return res, isNew, err
		}
	} else {
		ctx.Set(consts.UserID, userInfo.Id)
	}

	// 关联设备
	ud, err := u.ds.UserDeviceGetByDeviceUUID(ctx, sign.UUID)

	if err != nil && err == gorm.ErrRecordNotFound {
		privateKey, publicKey, _ := security.GenCurve25519Key()

		ud = entity.UserDevice{
			Id:                u.iSrv.GenSnowID(),
			UUID:              sign.UUID,
			AuthKey:           sign.AuthKey,
			ClientPublicKey:   sign.Key,
			ServicePrivateKey: base64.StdEncoding.EncodeToString(privateKey),
			ServicePublicKey:  base64.StdEncoding.EncodeToString(publicKey),
			UserId:            userInfo.Id,
		}
		err = u.dd.UserDeviceCreateNew(ctx, ud)
		if err != nil {
			return res, isNew, err
		}
	}
	if ud.ServicePrivateKey == "" || ud.ClientPublicKey != sign.Key {
		privateKey, publicKey, _ := security.GenCurve25519Key()
		ud.ServicePrivateKey = base64.StdEncoding.EncodeToString(privateKey)
		ud.ServicePublicKey = base64.StdEncoding.EncodeToString(publicKey)
		ud.ClientPublicKey = sign.Key
		ud.AuthKey = sign.AuthKey
		err = u.ds.UserDeviceUpdateByDeviceUUID(ctx, ud)
		if err != nil {
			return
		}
	}

	//role, err := u.UserGetSubscription(ctx, userInfo.Id)
	//if err != nil {
	//	return
	//}

	r := rand.New(rand.NewSource(time.Now().Unix()))
	num := r.Intn(100)
	settime := conf.AppConfig.Jwt.JwtTtl + int64(num*9)
	timeout := time.Duration(settime) * time.Second
	expireAt := time.Now().Add(timeout)
	claims := jwt.BuildClaims(expireAt, userInfo.Id, userInfo.Role, true, userInfo.IsAdministrator)
	token, err := jwt.GenToken(claims, conf.AppConfig.Jwt.Secret)
	if err != nil {
		logger.Infof("Jwt Token 生成错误：%s", userInfo.Username)
		return res, isNew, err
	}
	res.Token = token
	res.Role = claims.RoleId
	res.Timeout = int(settime) * 1000
	res.IsAnonymous = claims.IsAnonymousUser()
	res.IsAdministrator = claims.IsAdministrator()
	res.Key = ud.ServicePublicKey
	return
}

func (u *userService) makeAnonymousId(sign model.AnonymousAccessSign) string {
	deviceId := sign.BundleId + ":" + sign.UUID
	anonymousId := security.Md5WithSalt(deviceId, consts.RegisterAnonymousSlat)
	anonymousId = "$TAnonymous:" + anonymousId
	return anonymousId
}

// 生成一个匿名用户，并存储在数据库
func (u *userService) genAnonymousUser(ctx *gin.Context, anonymousId string) (*entity.User, error) {
	user := &entity.User{}
	user.Id = u.iSrv.GenSnowID()
	ctx.Set(consts.UserID, user.Id)
	user.Username = anonymousId
	randStr := u.iSrv.GenSnowStr()
	nickname := randStr[len(randStr)/2:]
	user.Nickname = nickname
	user.RegisteredIp = ctx.ClientIP()
	user.Role = consts.StandardUser
	user.IsAnonymous = true
	err := u.ud.UserCreate(ctx, user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (u *userService) UserLogout(ctx context.Context, tokenstr string) error {
	return jwt.JoinBlackList(ctx, tokenstr, conf.AppConfig.Jwt.Secret)
}

func (u *userService) UserRefresh(ctx *gin.Context) (res model.UserLoginRes, err error) {
	userId := ctx.GetInt64(consts.UserID)
	tokenStr := ctx.GetString(consts.JWTTokenCtx)
	userInfo, err := u.ud.UserGetById(ctx, userId)
	if err != nil {
		logger.Infof("查询用户失败：%s", err)
		return res, err
	}
	if userInfo.UserId == 0 {
		logger.Infof("用户不存在：%d", userId)
		return res, errors.New(fmt.Sprintf("用户不存在：%d", userId))
	}
	if userInfo.IsActive != true {
		err = errors.New("用户未激活!")
		return res, err
	}
	r := rand.New(rand.NewSource(time.Now().Unix()))
	num := r.Intn(100)
	settime := conf.AppConfig.Jwt.JwtTtl + int64(num*9)
	timeOut := time.Duration(settime) * time.Second
	expireAt := time.Now().Add(timeOut)
	claims := jwt.BuildClaims(expireAt, userId, userInfo.Role, userInfo.IsAnonymous, userInfo.IsAdministrator)
	token, err := jwt.GenToken(claims, conf.AppConfig.Jwt.Secret)
	if err != nil {
		logger.Infof("Jwt Token 生成错误：%s", userInfo.Username)
		return res, err
	}

	res.Token = token
	res.Role = claims.RoleId
	res.IsAnonymous = claims.IsAnonymousUser()
	res.IsAdministrator = claims.IsAdministrator()
	res.Timeout = int(settime) * 1000
	err = jwt.JoinBlackList(ctx, tokenStr, conf.AppConfig.Jwt.Secret)
	if err != nil {
		logger.Infof("加入黑名单失败：%s", userInfo.Username)
	}
	return res, err
}

func (u *userService) UserAuthStatus(ctx *gin.Context) (res model.UserAuthStatusRes, err error) {
	tokenStr := ctx.GetString(consts.JWTTokenCtx)
	if err != nil {
		res.IsInvalid = true
		return
	}
	claims, err := jwt.ParseToken(tokenStr, conf.AppConfig.Jwt.Secret)
	if err != nil || claims.UserId == 0 {
		res.IsInvalid = true
		return res, errors.New(fmt.Sprintf("无效的token：%s", err.Error()))
	}
	res.IsInvalid = false
	return
}

func (u *userService) UserGetInfo(ctx context.Context, userId int64) (res model.UserGetInfoRes, err error) {
	// 根据userId查询redis中的缓存
	rdsKey := consts.UserInfoPrefix + strconv.FormatInt(userId, 10)
	jsonBytes, err := u.rc.Get(ctx, rdsKey).Bytes()
	if err == nil {
		err = json.Unmarshal(jsonBytes, &res)
		if err == nil {
			res.UserId = userId
			return res, nil
		}
		logger.Errorf("UserInfoRes反序列化失败:%v", err.Error())
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		logger.Debugf("UserInfoRes缓存不存在:%v", err.Error())
	}

	user, err := u.ud.UserGetById(ctx, userId)
	if err != nil {
		return res, err
	}
	res.Email = user.Email
	res.Role = user.Role
	res.RoleName = consts.RoleToString[user.Role]
	res.Nickname = user.Nickname
	res.Phone = user.Phone
	res.Username = user.Username
	res.UserId = user.UserId

	res.AvatarUrl = user.AvatarUrl

	jsonBytes, err = json.Marshal(res)
	if err != nil {
		logger.Errorf("UserInfoRes序列化失败:%v", err.Error())
		return res, err
	}
	// 存储到redis中 10分钟过期
	err = u.rc.Set(ctx, rdsKey, jsonBytes, consts.RedisExrDefault).Err()
	if err != nil {
		logger.Errorf("UserInfoRes存储Cache失败:%v", err.Error())
		return res, nil
	}
	return res, nil
}

func (u *userService) UserGetAvatar(ctx *gin.Context) (res model.UserAvatarRes, err error) {
	userId := ctx.GetInt64(consts.UserID)
	rdsKey := consts.UserAvatarPrefix + ":1:" + strconv.FormatInt(userId, 10)
	// 先从redis中查找用户头像
	avatarUrl, err := u.rc.Get(ctx, rdsKey).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		// 从数据库中查找
		avatarUrl, err = u.ud.UserGetAvatar(ctx, userId)
		if err != nil {
			return
		}
		// 将用户头像保存到redis
		err = u.rc.SetNX(ctx, rdsKey, avatarUrl, 0).Err()
		if err != nil {
			return
		}
	}
	if avatarUrl == "" {
		return res, err
	}
	data, err := os.ReadFile(avatarUrl)
	if err != nil {
		return
	}
	res.Avatar = base64.StdEncoding.EncodeToString(data)
	res.AvatarURI = avatarUrl
	return
}

func (u *userService) UserBillGet(ctx *gin.Context, req model.UserBillGetReq) (res model.UserBillListRes, err error) {
	userId := ctx.GetInt64(consts.UserID)
	date := time.Unix(int64(req.EndTime)/1000, 0)
	if req.EndTime == 0 {
		date = time.Now().AddDate(0, 0, 1) // 往后查询一天，否则无法查询当天的记录
	}
	end := date.Local().Format(consts.DateLayout)
	// 查询从date开始，一天的账单
	//end := date.AddDate(0, 0, 1).Local().Format(consts.DateLayout)
	// 查询从date开始，一年前的账单
	var start string
	if req.Days > 0 {
		start = date.AddDate(0, 0, -req.Days).Local().Format(consts.DateLayout)
	} else {
		start = date.AddDate(-1, 0, 0).Local().Format(consts.DateLayout)
	}

	plan, err := u.UserGetPlan(ctx, userId)
	//balance, err := u.UserGetBalance(ctx, userId)
	if err != nil {
		return
	}
	bills, err := u.ud.UserBillGet(ctx, userId, req.Page, req.PageSize, start, end)
	if err != nil {
		return
	}
	res.BillList = bills
	res.Plan = plan
	return res, nil
}

func (u *userService) UserBillGetTransactionId(ctx *gin.Context, transactionId string) (res entity.Bill, err error) {
	userId := ctx.GetInt64(consts.UserID)
	res, err = u.ud.UserBillGetTransactionId(ctx, userId, transactionId)
	return
}

func (u *userService) UserGetBalance(ctx context.Context, userId int64) (res model.UserBalanceGetRes, err error) {
	// 从缓存中查找余额
	cacheKey := consts.UserBalancePrefix + strconv.FormatInt(userId, 10)
	balance, err := u.rc.Get(ctx, cacheKey).Float64()
	if err == nil {
		res = model.NewUserBalanceGetRes(balance)
		return
	}
	if err != redis.Nil {
		logger.Errorf("Redis 连接异常：%v", err.Error())
	}
	logger.Debugf("用户balance缓存不存在：%v", err.Error())
	// 从数据库中查找余额
	balance, err = u.ud.UserGetBalance(ctx, userId)
	if err != nil {
		return res, err
	}
	// 设置到缓存
	err = u.rc.Set(ctx, cacheKey, balance, consts.RedisExrDefault).Err()
	if err != nil {
		logger.Errorf("用户balance缓存失败：%v", err.Error())
	}
	res = model.NewUserBalanceGetRes(balance)
	return res, nil
}

func (u *userService) UserGetPlan(ctx context.Context, userId int64) (res model.UserPlanGetRes, err error) {
	// 从缓存中查找余额
	balanceRes, err := u.UserGetBalance(ctx, userId)
	if err != nil {
		return
	}
	// 从缓存中查找订阅
	subscription, err := u.UserGetSubscription(ctx, userId)
	if err != nil {
		return
	}
	res.Balance = balanceRes
	res.Subscription = subscription
	return
}

func (u *userService) UserBalanceChange(ctx context.Context, userId int64, billType consts.BillType, oldbalance, amount float64, comment string, cdKeyId *int64, order *model.UserBillExtras, originBill *entity.Bill) (err error) {
	newBalance := oldbalance + amount
	user := entity.User{}
	user.Id = userId
	user.Balance = newBalance
	// 更新用户余额
	err = u.ud.UserUpdate(ctx, &user)
	if err != nil {
		return err
	}
	bill := entity.Bill{}
	bill.Id = u.iSrv.GenSnowID()
	bill.CostChange = amount
	bill.Balance = newBalance
	bill.UserId = userId
	bill.CostComment = comment
	bill.BillType = int(billType)
	if originBill != nil {
		bill.OriginalBillId = &originBill.Id
	}
	if order != nil {
		bill.TransactionId = order.Transaction.TransactionId
		bill.OriginalTransactionId = order.Transaction.OriginalTransactionId
		extras, err := json.Marshal(order)
		if err != nil {
			return err
		}
		bill.Extras = datatypes.JSON(extras)
	}
	if cdKeyId != nil {
		bill.CdKeyId = cdKeyId
	}
	err = u.ud.UserBillCreate(ctx, &bill)
	if err != nil {
		logger.Errorf("UserBillCreate失败:%v", err.Error())
	}
	// 将余额设置到缓存
	err = u.rc.SetXX(ctx, consts.UserBalancePrefix+strconv.FormatInt(userId, 10), newBalance, 0).Err()
	if err != nil {
		logger.Errorf("UserBalance更新存储Cache失败:%v", err.Error())
	}
	return nil
}

// 用户订阅，创建账单
func (u *userService) UserBillUpdateSubscribe(ctx context.Context, userId int64, subscribe entity.SubscribeProduct, order *model.UserBillExtras) (err error) {
	// 更新用户角色
	var user entity.User
	user.Id = userId
	user.Role = subscribe.Role
	err = u.ud.UserUpdate(ctx, &user)
	if err != nil {
		return
	}

	bill := entity.Bill{}
	bill.Id = u.iSrv.GenSnowID()
	bill.CostChange = subscribe.Price
	bill.UserId = userId
	bill.BillType = int(consts.BillTypeMemberRecharge)
	bill.CostComment = fmt.Sprint(1) // 作为单位存储 一个月的plus
	bill.SubscribeProductId = &subscribe.Id
	if order != nil {
		bill.TransactionId = order.Transaction.TransactionId
		bill.OriginalTransactionId = order.Transaction.OriginalTransactionId
		extras, err := json.Marshal(order)
		if err != nil {
			return err
		}
		bill.Extras = datatypes.JSON(extras)
	}
	err = u.ud.UserBillCreate(ctx, &bill)
	if err != nil {
		logger.Errorf("UserBillCreate失败:%v", err.Error())
	}

	// 删除缓存中的用户角色
	rdsKey := consts.UserSubscriptionPrefix + strconv.FormatInt(userId, 10)
	u.rc.Del(ctx, rdsKey)

	return nil
}

// 用户订阅退款，创建账单
func (u *userService) UserBillUpdateSubscribeRefund(ctx context.Context, userId int64, subscribe entity.SubscribeProduct, order *model.UserBillExtras, origBill entity.Bill) (err error) {

	bill := entity.Bill{}
	bill.Id = u.iSrv.GenSnowID()
	bill.CostChange = subscribe.Price
	bill.UserId = userId
	bill.BillType = int(consts.BillTypeMemberRefund)
	bill.CostComment = "Apple inApp subscription plus refund"
	bill.SubscribeProductId = &subscribe.Id
	bill.OriginalBillId = &origBill.Id
	if order != nil {
		bill.TransactionId = order.Transaction.TransactionId
		bill.OriginalTransactionId = order.Transaction.OriginalTransactionId
		extras, err := json.Marshal(order)
		if err != nil {
			return err
		}
		bill.Extras = datatypes.JSON(extras)
	}
	err = u.ud.UserBillCreate(ctx, &bill)
	if err != nil {
		logger.Errorf("UserBillCreate失败:%v", err.Error())
	}

	// 删除缓存中的用户角色
	rdsKey := consts.UserSubscriptionPrefix + strconv.FormatInt(userId, 10)
	u.rc.Del(ctx, rdsKey)

	return nil
}

func (u *userService) UserVerifyEmail(ctx *gin.Context, email string) (res model.UserVerifyEmailRes, err error) {
	userId, err := u.ud.UserVerifyEmail(ctx, email)
	if err != nil {
		return
	}
	if userId != 0 {
		res.IsValid = false
		ctx.Set(consts.UserID, userId)
		logger.Debugf("邮箱校验信息：%d", userId)
	} else {
		if conf.AppConfig.Email.PreCheck {
			mVeri := mail.NewVerifier()
			err := mVeri.VerifierEmail(email)
			if err != nil {
				logger.Warnf("邮箱%s验证码错误：%v", email, err)
				res.IsValid = false
				return res, err
			}
		}
		res.IsValid = true
	}
	logger.Debugf("邮箱校验信息：%d", userId)
	return
}

func (u *userService) UserVerifyUserName(ctx context.Context, username string) (res model.UserVerifyUsernameRes, err error) {
	count, err := u.ud.UserVerifyUsername(ctx, username)
	if err != nil {
		return
	}
	if count != 0 {
		res.IsValid = false
	} else {
		res.IsValid = true
	}
	return
}

func (u *userService) UserUpdateNickName(ctx context.Context, userId int64, nickname string) (res model.UserUpdateNicknameRes, err error) {
	err = u.ud.UserUpdateNickName(ctx, userId, nickname)
	if err != nil {
		res.IsChanged = false
	} else {
		res.IsChanged = true
	}
	return
}

func (u *userService) UserSubscribeRole(ctx context.Context, userId int64, roleType int, identifier string) (uSubscribe *entity.UserSubscriptions, err error) {
	if roleType != consts.StandardUser &&
		roleType != consts.PlusMember &&
		roleType != consts.Enterprise {
		return nil, errors.New("Role is valid")
	}
	//expiresAt := time.UnixMilli(expiresDate)
	uSubscribe, err = u.ud.UserSubscriptionsGet(ctx, userId)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	if uSubscribe == nil || uSubscribe.Id == 0 {
		// 创建用户订阅
		subscribeProduct, err := u.ud.SubscribeProductGet(ctx, identifier)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		uSubscribe = &entity.UserSubscriptions{
			Id:                 u.iSrv.GenSnowID(),
			UserId:             userId,
			SubscribeProductID: subscribeProduct.Id,
			SubscribeProduct:   subscribeProduct,
			StartedAt:          utils.JsonTime(now),
			ExpiredAt:          utils.JsonTime(now.AddDate(0, 0, subscribeProduct.Days)),
		}
		err = u.ud.UserSubscriptionsCreate(ctx, uSubscribe)
	} else {
		// 更新用户订阅
		origExpiredAt := time.Time(uSubscribe.ExpiredAt)
		now := time.Now()
		var expiredAt time.Time
		if origExpiredAt.After(now) {
			// 如果在当前时间后面，则增加到订阅时间
			expiredAt = origExpiredAt.AddDate(0, 0, uSubscribe.SubscribeProduct.Days)
		} else {
			expiredAt = now.AddDate(0, 0, uSubscribe.SubscribeProduct.Days)
		}
		uSubscribe.ExpiredAt = utils.JsonTime(expiredAt)
		err = u.ud.UserSubscriptionsUpdate(ctx, uSubscribe)
	}
	if err != nil {
		return nil, err
	}

	rdsKey := consts.UserInfoPrefix + strconv.FormatInt(userId, 10)
	u.rc.Del(ctx, rdsKey)
	return uSubscribe, nil
}

func (u *userService) UserSubscribeRoleRefund(ctx context.Context, userId int64, roleType int) (uSubscribe *entity.UserSubscriptions, err error) {
	if roleType != consts.StandardUser &&
		roleType != consts.PlusMember &&
		roleType != consts.Enterprise {
		return nil, errors.New("Role is valid")
	}

	uSubscribe, err = u.ud.UserSubscriptionsGet(ctx, userId)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	// 更新用户订阅
	origExpiredAt := time.Time(uSubscribe.ExpiredAt)
	now := time.Now()
	var expiredAt time.Time
	if origExpiredAt.After(now) {
		// 如果在当前时间后面，则退款到订阅时间
		expiredAt = origExpiredAt.AddDate(0, 0, -uSubscribe.SubscribeProduct.Days)
	}
	uSubscribe.ExpiredAt = utils.JsonTime(expiredAt)
	err = u.ud.UserSubscriptionsUpdate(ctx, uSubscribe)
	if err != nil {
		return nil, err
	}

	// 更新用户角色
	var user entity.User
	user.Id = userId
	if uSubscribe.IsExpired() {
		// 国期，恢复为普通用户
		user.Role = consts.StandardUser
	}
	err = u.ud.UserUpdate(ctx, &user)
	if err != nil {
		return
	}

	rdsKey := consts.UserInfoPrefix + strconv.FormatInt(userId, 10)
	u.rc.Del(ctx, rdsKey)
	return uSubscribe, nil
}

func (u *userService) UserPasswordVerify(ctx *gin.Context, password string) (isValid bool) {
	userId := ctx.GetInt64(consts.UserID)
	isValid = false
	user, err := u.ud.UserGetById(ctx, userId)
	if err != nil {
		logger.Errorf("查询用户失败：%v", err.Error())
		return
	}
	if !security.ValidatePassword(security.PasswordDecryption(password, consts.CBCKEY), user.Password) {
		logger.Infof("密码错误%s", user.Username)
		return
	}
	isValid = true
	return
}

func (u *userService) UserPasswordModify(ctx *gin.Context, password string) (err error) {
	userId := ctx.GetInt64(consts.UserID)
	user := entity.User{Id: userId}
	user.Password, err = security.Encrypt(security.PasswordDecryption(password, consts.CBCKEY))
	if err != nil {
		return err
	}
	err = u.ud.UserUpdate(ctx, &user)
	return
}

func (u *userService) UserPasswordForget(ctx *gin.Context) (err error) {
	tempcode, email, nikcname, err := u.UserTempCodeGen(ctx)
	//19+16 35
	err = mail.SendForgetCode(email, nikcname, tempcode)
	return
}

func (u *userService) UserActiveGen(ctx *gin.Context) (err error) {
	tempcode, email, nickname, err := u.UserTempCodeGen(ctx)
	if err != nil {
		logger.Errorf("激活码生成错误：%v", err.Error())
		return
	}
	err = mail.SendActiceCode(email, nickname, tempcode)
	return
}

func (u *userService) UserInviteLinkGet(ctx *gin.Context) (res model.UserInviteLinkRes, err error) {
	userId := ctx.GetInt64(consts.UserID)
	jsonbyte, err := u.rc.Get(ctx, consts.UserInviteLinkPrefix+strconv.FormatInt(userId, 10)).Bytes()
	if err == nil {
		err = json.Unmarshal(jsonbyte, &res)
		if err == nil {
			return res, nil
		}
		logger.Errorf("UserInviteLinkRes反序列化失败:%v", err.Error())
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		logger.Debugf("UserInviteLinkRes缓存不存在:%v", err.Error())
	}
	invite, err := u.ud.UserInviteGetByUser(ctx, userId)
	if err != nil {
		return
	}
	code := invite.InviteCode
	if code == "" {
		for i := 1; i <= 3; i++ {
			code, err = u.UserInviteGen(ctx)
			if err == nil {
				break
			}
			if i == 3 {
				break
			}
			time.Sleep(1)
		}
	}
	externalURL := conf.AppConfig.ExternalURL
	res.InviteLink = externalURL + "#/register/" + code
	res.InviteNumber = invite.InviteNumber
	res.InviteReward = float64(invite.InviteNumber * consts.InviteReward)
	jsonbyte, err = json.Marshal(res)
	if err != nil {
		logger.Errorf("UserInviteLinkRes序列化失败:%v", err.Error())
		return res, nil
	}
	err = u.rc.Set(ctx, consts.UserInviteLinkPrefix+strconv.FormatInt(userId, 10), jsonbyte, 0).Err()
	if err != nil {
		logger.Errorf("UserInviteLinkRes存储Cache失败:%v", err.Error())
		return res, nil
	}
	return
}

func (u *userService) UserInviteGen(ctx *gin.Context) (string, error) {
	codeId := u.iSrv.GenSnowID()
	userId := ctx.GetInt64(consts.UserID)
	invite := &entity.Invite{}
	invite.Id = codeId
	invite.UserId = userId
	invite.InviteCode = uuid.GetInvCodeByUID(codeId)
	return invite.InviteCode, u.ud.UserInviteGen(ctx, invite)
}

func (u *userService) UserInviteReward(ctx *gin.Context) {
	current_userId := ctx.GetInt64(consts.UserID)

	invite_str, err := u.rc.Get(ctx, consts.UserInvitePrefix+strconv.FormatInt(current_userId, 10)).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		logger.Debugf("用户：%d,为使用邀请码注册", current_userId)
		return
	}
	if invite_str == "" || invite_str == "0" { // 没有邀请人
		// logger.Errorf("UserID：%v",)
		return
	}
	invite_userId, err := strconv.ParseInt(invite_str, 10, 64)
	if err != nil {
		logger.Errorf("invite_userId:%v 序列化失败", invite_str)
		return
	}
	inviteuser_banlance, err := u.UserGetBalance(ctx, invite_userId)
	if err != nil {
		logger.Errorf("invite_userId:%v 获取用户余额失败", invite_userId)
		return
	}
	err = u.UserBalanceChange(ctx, invite_userId, consts.BillTypePointRewardByInvite, inviteuser_banlance.Balance, consts.InviteReward, "奖励-邀请新用户成功", nil, nil, nil)
	if err != nil {
		logger.Errorf("UserID：%v 获取奖励失败", invite_userId)
		return
	}
	currentuser_banlance, err := u.UserGetBalance(ctx, current_userId)
	if err != nil {
		logger.Errorf("invite_userId:%v 获取用户余额失败", current_userId)
		return
	}
	err = u.UserBalanceChange(ctx, current_userId, consts.BillTypePointRewardByInvited, currentuser_banlance.Balance, consts.InviteReward, "奖励-受邀请注册", nil, nil, nil)
	if err != nil {
		logger.Errorf("current_userId:%v 获取奖励失败", current_userId)
		return
	}
	invite, err := u.ud.UserInviteGetByUser(ctx, invite_userId)
	if err != nil {
		logger.Errorf("UserID：%v 获取Invite信息失败", invite_userId)
		return
	}
	invite.InviteNumber += 1
	err = u.ud.UserInviteUpdate(ctx, &entity.Invite{Id: invite.Id, InviteNumber: invite.InviteNumber})
	if err != nil {
		logger.Errorf("UserID：%v 更新邀请次数失败", invite_userId)
		return
	}
	err = u.rc.Del(ctx, consts.UserInviteLinkPrefix+strconv.FormatInt(invite_userId, 10)).Err()
	if err != nil {
		logger.Errorf("删除UserInviteLinkRes缓存失败:%v", err.Error())
	}
	return
}

func (u *userService) UserInviteVerify(ctx *gin.Context, code string) {
	current_userId := ctx.GetInt64(consts.UserID)
	invite, err := u.ud.UserInviteGetByCode(ctx, code)
	if err != nil {
		logger.Errorf("current_userId:%v 邀请码错误", current_userId)
		return
	}
	invite_userId := invite.UserId

	timer := 172800 * time.Second
	err = u.rc.SetNX(ctx, consts.UserInvitePrefix+strconv.FormatInt(current_userId, 10), invite_userId, timer).Err()
	if err != nil {
		return
	}
	return
}

func (u *userService) UserActiveChange(ctx *gin.Context) (err error) {
	userId := ctx.GetInt64(consts.UserID)
	user := entity.User{}
	user.Id = userId
	user.IsActive = true
	err = u.ud.UserUpdate(ctx, &user)
	if err != nil {
		return
	}
	// 更新用户注册奖励的余额
	_ = u.UserBalanceChange(ctx, userId, consts.BillTypePointRewardByRegister, 0, consts.RegisterReward, "奖励-新用户注册", nil, nil, nil)
	for i := 1; i <= 3; i++ {
		// 为当前用生成邀请码，失败会重试三次
		_, err = u.UserInviteGen(ctx)
		if err == nil {
			break
		}
		if i == 3 {
			break
		}
		time.Sleep(1)
	}
	return nil
}

func (u *userService) UserActiveVerify(ctx *gin.Context) bool {
	userId := ctx.GetInt64(consts.UserID)
	user, err := u.ud.UserGetById(ctx, userId)
	if err != nil {
		logger.Errorf("查询用户失败：%v", err.Error())
		return false
	}
	if user.IsActive != true {
		return false
	}
	return true
}

func (u *userService) UserTempCodeVerify(ctx *gin.Context, tempcode string) (Isvalid bool) {
	Isvalid = false
	codeStr, err := base64.StdEncoding.DecodeString(tempcode)
	if err != nil {
		return
	}
	codelist := strings.Split(string(codeStr), "|")
	if len(codelist) < 2 {
		// err = errors.New("Active Failed")
		return
	}
	code := codelist[0]
	username := codelist[1]
	userInfo, err := u.ud.UserGetByName(ctx, username)
	if err != nil {
		return
	}
	ctx.Set(consts.UserID, userInfo.Id)
	a := active.ActiveCodeCompare(ctx, code, userInfo.Id)
	if !a {
		Isvalid = false
	} else {
		Isvalid = true
	}
	return
}

func (u *userService) UserTempCodeGen(ctx *gin.Context) (tempcode string, email string, nickname string, err error) {

	userId := ctx.GetInt64(consts.UserID)
	user, err := u.ud.UserGetById(ctx, userId)
	code, err := active.ActiveCodeGen(ctx, userId)
	if err != nil {
		return
	}
	email = user.Email
	nickname = user.Nickname
	tempcode = base64.StdEncoding.EncodeToString([]byte(code + "|" + user.Username))
	return
}

// 用内购订阅演唱会员时长
func (u *userService) inAppsSubscriptionRole(ctx *gin.Context, extras model.UserBillExtras, userId int64) (err error) {
	groupName := u.groupNameByProductId(extras.Transaction.ProductId)
	if strings.ToLower(groupName) == "plus" {
		// 更新用户角色为标准会员
		uSubscribe, err := u.UserSubscribeRole(ctx, userId, consts.PlusMember, extras.Transaction.ProductId)
		if err != nil {
			return err
		}

		// 更新账单
		err = u.UserBillUpdateSubscribe(ctx, userId, uSubscribe.SubscribeProduct, &extras)
		return err
	} else {
		return errors.New("Unsupported inApp group: " + groupName)
	}
}

// 用内购订阅演唱会员时长
func (u *userService) inAppsSubscriptionRoleRefund(ctx *gin.Context, extras model.UserBillExtras, userId int64, origBill entity.Bill) (err error) {
	groupName := u.groupNameByProductId(extras.Transaction.ProductId)
	if strings.ToLower(groupName) == "plus" {
		// 更新用户角色为标准会员
		uSubscribe, err := u.UserSubscribeRoleRefund(ctx, userId, consts.PlusMember)
		if err != nil {
			return err
		}

		// 更新账单
		err = u.UserBillUpdateSubscribeRefund(ctx, userId, uSubscribe.SubscribeProduct, &extras, origBill)
		return err
	} else {
		return errors.New("Unsupported inApp group: " + groupName)
	}
}

func (u *userService) groupNameByProductId(productId string) string {
	components := strings.Split(productId, ".")
	for i, item := range components {
		if strings.ToLower(item) == "subscription" {
			if i+1 < len(components) {
				return components[i+1]
			}
		}
	}
	return ""
}

func (u *userService) CaptchaGen(ctx *gin.Context) (res model.CaptchaRes, err error) {
	image, err := verification.GenerateCaptcha(ctx)
	res.Image = image
	return
}

func (u *userService) CaptchaVerify(ctx *gin.Context, code string) bool {
	return verification.VerifyCaptcha(ctx, code)
}

func (u *userService) UserLogSave(ctx *gin.Context, operation, business string) error {
	var req struct {
		Log model.UserLogDeviceInfo `json:"log" validate:"required"`
	}
	if err := ctx.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		var jsonErr *json.UnmarshalTypeError
		if errors.As(err, &jsonErr) {
			log.Println("Json binding error")
			// 当json绑定错误时，如果绑定成功了deviceId，则认为成功
			if req.Log.DeviceId == "" {
				return err
			}
		}
	}
	device := req.Log
	userId := ctx.GetInt64(consts.UserID)
	uLog := entity.UserLog{}
	uLog.Id = u.iSrv.GenSnowID()
	uLog.UserId = userId
	uLog.UserIP = ctx.ClientIP()
	uLog.Operation = operation
	uLog.Business = business
	uLog.ScreenWidth = device.ScreenWidth
	uLog.ScreenHeight = device.ScreenHeight
	uLog.OsVersion = device.OsVersion
	uLog.Os = device.Os
	uLog.AppBuildNumber = device.AppBuildNumber
	uLog.AppPlatform = device.AppPlatform
	uLog.AppVersion = device.AppVersion
	uLog.AppPackageId = device.AppPackageId
	uLog.Radio = device.Radio
	uLog.Carrier = device.Carrier
	uLog.IsWifi = device.IsWifi
	uLog.Proxy = device.Proxy
	uLog.DeviceId = device.DeviceId
	uLog.DeviceBrand = device.DeviceBrand
	uLog.DeviceModelDesc = device.DeviceModelDesc
	uLog.DeviceModel = device.DeviceModel
	languageId := device.LanguageId
	if languageId == "" {
		languageId = ctx.GetString(consts.LanguageId)
	}
	uLog.LanguageId = languageId
	err := u.ud.UserAddLog(ctx, &uLog)
	return err
}
