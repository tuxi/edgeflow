package service

import (
	"context"
	"edgeflow/internal/consts"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/logger"
	"edgeflow/utils/uuid"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"time"
)

var _ DeviceService = (*deviceService)(nil)

type DeviceService interface {
	UserGetDeviceTokenList(ctx context.Context) ([]entity.DeviceToken, error)
	UserDeviceTokenUpdate(ctx *gin.Context, req model.DeviceTokenReportReq) error
	UserDeviceUpdate(ctx *gin.Context, req model.DeviceReportReq) error
	UserGetDeviceTokenByDeviceUUID(ctx context.Context, deviceUUID string) (entity.DeviceToken, error)
	UserGetDeviceTokenList1(ctx context.Context) (res model.DeviceTokenListRes, err error)

	// 根据设备解密数据
	UserDeviceDecrypt(ctx *gin.Context, req model.ChaChaPolyEncryptData) (payloadData []byte, err error)
	// 根据设备加密数据
	UserDeviceEncrypt(ctx *gin.Context, payload interface{}) (encryptData model.ChaChaPolyEncryptData, err error)

	UserDeviceGetByDeviceUUID(ctx *gin.Context, uuid string) (entity.UserDevice, error)
	UserDeviceUpdateByDeviceUUID(ctx *gin.Context, ud entity.UserDevice) error
}

type deviceService struct {
	dd   dao.DeviceDao
	iSrv *uuid.SnowNode
	rc   *redis.Client
}

func NewService(dd dao.DeviceDao) *deviceService {
	return &deviceService{
		dd:   dd,
		iSrv: uuid.NewNode(3),
		rc:   cache.GetRedisClient(),
	}
}

func (u *deviceService) UserGetDeviceTokenList(ctx context.Context) ([]entity.DeviceToken, error) {
	tokens, err := u.dd.UserDeviceTokenGetList(ctx)
	if err != nil {
		logger.Errorf("获取所有可推送的用户失败：%v", err.Error())
		return nil, err
	}
	return tokens, err
}

func (u *deviceService) UserGetDeviceTokenByDeviceUUID(ctx context.Context, deviceUUID string) (entity.DeviceToken, error) {
	token, err := u.dd.UserDeviceTokenGetByDeviceUUID(ctx, deviceUUID)
	if err != nil {
		logger.Errorf("根据deviceKey：%v 获取devicetoken失败", deviceUUID)
		return token, err
	}
	return token, nil
}

func (u *deviceService) UserDeviceTokenUpdate(ctx *gin.Context, req model.DeviceTokenReportReq) error {
	deviceToken, err := u.dd.UserDeviceTokenGetByDeviceUUID(ctx, req.DeviceUUID)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Debugf("查询device token记录出错:%v", err.Error())
			return err
		}
		// 不存在记录，创建device token
		deviceToken = entity.DeviceToken{
			Id:          u.iSrv.GenSnowID(),
			DeviceToken: req.DeviceToken,
			DeviceUUID:  req.DeviceUUID,
			Platform:    req.Platform,
		}
		err = u.dd.UserDeviceTokenCreateNew(ctx, deviceToken)
		if err != nil {
			return err
		}
	} else {
		// 已存在，则更新device token
		err := u.dd.UserDeviceTokenUpdateByDeviceUUID(ctx, req.DeviceUUID, req.DeviceToken)
		if err != nil {
			logger.Errorf("更新device token 失败：%v", err.Error())
			return err
		}
	}
	userDevice, err := u.UserDeviceGetByDeviceUUID(ctx, deviceToken.DeviceUUID)
	var previousDeviceTokenId int64
	if userDevice.DeviceTokenId != nil {
		previousDeviceTokenId = *userDevice.DeviceTokenId
	}
	if err == nil && previousDeviceTokenId != deviceToken.Id {
		userDevice.DeviceTokenId = &deviceToken.Id
		// 如果关联关系已经存在，我们不需要做任何操作
		err = u.UserDeviceUpdateByDeviceUUID(ctx, userDevice)
		if err != nil {
			return err
		}
	}
	return nil
}

func (u *deviceService) UserDeviceUpdate(ctx *gin.Context, req model.DeviceReportReq) error {
	device, err := u.dd.DeviceGetByUUID(ctx, req.UUID)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Debugf("查询device 记录出错:%v", err.Error())
			return err
		}
		// 不存在记录，创建device token
		device = entity.Device{
			Id:              u.iSrv.GenSnowID(),
			UUID:            req.UUID,
			ScreenHeight:    req.ScreenHeight,
			ScreenWidth:     req.ScreenWidth,
			Os:              req.Os,
			OsVersion:       req.OsVersion,
			AppBuildNumber:  req.AppBuildNumber,
			AppVersion:      req.AppVersion,
			AppPlatform:     req.AppPlatform,
			AppPackageId:    req.AppPackageId,
			DeviceBrand:     req.DeviceBrand,
			DeviceModelDesc: req.DeviceModelDesc,
			DeviceModel:     req.DeviceModel,
			Radio:           req.Radio,
			Carrier:         req.Carrier,
			IsWifi:          req.IsWifi,
			Proxy:           req.Proxy,
			LanguageId:      req.LanguageId,
			ClientIP:        ctx.ClientIP(),
			SerialNumber:    req.SerialNumber,
			PlatformUUID:    req.PlatformUUID,
		}
		err = u.dd.DeviceCreateNew(ctx, device)
		if err != nil {
			return err
		}
	} else {
		device.ScreenHeight = req.ScreenHeight
		device.ScreenWidth = req.ScreenWidth
		device.Os = req.Os
		device.OsVersion = req.OsVersion
		device.AppBuildNumber = req.AppBuildNumber
		device.AppVersion = req.AppVersion
		device.AppPlatform = req.AppPlatform
		device.AppPackageId = req.AppPackageId
		device.DeviceBrand = req.DeviceBrand
		device.DeviceModelDesc = req.DeviceModelDesc
		device.DeviceModel = req.DeviceModel
		device.Radio = req.Radio
		device.Carrier = req.Carrier
		device.IsWifi = req.IsWifi
		device.Proxy = req.Proxy
		device.LanguageId = req.LanguageId
		device.ClientIP = ctx.ClientIP()
		// 已存在，则更新device token
		err := u.dd.DeviceUpdateByUUID(ctx, device)
		if err != nil {
			logger.Errorf("更新device token 失败：%v", err.Error())
			return err
		}
	}

	// 如果用户已登录，查找并更新用户设备关联关系
	userDevice, err := u.UserDeviceGetByDeviceUUID(ctx, device.UUID)
	var previousDeviceId int64
	if userDevice.DeviceId != nil {
		previousDeviceId = *userDevice.DeviceId
	}
	if err == nil && previousDeviceId != device.Id {
		userDevice.DeviceId = &device.Id
		// 如果关联关系已经存在，我们不需要做任何操作
		err = u.UserDeviceUpdateByDeviceUUID(ctx, userDevice)
		if err != nil {
			return err
		}
	}
	return nil
}

func (u *deviceService) UserGetDeviceTokenList1(ctx context.Context) (res model.DeviceTokenListRes, err error) {
	tokens, err := u.UserGetDeviceTokenList(ctx)
	if err != nil {
		logger.Errorf("查询所有deviceToken失败：%v", err.Error())
		return
	}
	var m model.DeviceTokenOne
	var devices []model.DeviceTokenOne
	for _, item := range tokens {
		m.DeviceUUID = item.DeviceUUID
		m.CreatedAt = item.CreatedAt
		m.UpdatedAt = item.UpdatedAt
		m.Platform = item.Platform
		devices = append(devices, m)
	}
	res.Devices = devices
	return res, nil
}

func (ds *deviceService) UserDeviceDecrypt(ctx *gin.Context, req model.ChaChaPolyEncryptData) (payloadData []byte, err error) {
	deviceId := ctx.GetString(consts.DeviceId)
	ud, err := ds.UserDeviceGetByDeviceUUID(ctx, deviceId)
	if err != nil {
		return
	}

	payloadData, err = ud.UserDeviceDecrypt(req.S, req.N, req.Payload)
	return
}
func (ds *deviceService) UserDeviceEncrypt(ctx *gin.Context, payload interface{}) (encryptData model.ChaChaPolyEncryptData, err error) {
	deviceId := ctx.GetString(consts.DeviceId)
	ud, err := ds.UserDeviceGetByDeviceUUID(ctx, deviceId)
	if err != nil {
		return
	}

	s, n, content, err := ud.UserDeviceEncrypt(payload)
	if err != nil {
		return
	}
	encryptData.S = s
	encryptData.N = n
	encryptData.Payload = content
	return
}

func (ds *deviceService) UserDeviceGetByDeviceUUID(ctx *gin.Context, uuid string) (entity.UserDevice, error) {
	var userDevice entity.UserDevice
	rdsKey := consts.UserDeviceInfoPrefix + uuid
	jsonBytes, err := ds.rc.Get(ctx, rdsKey).Bytes()
	if err == nil {
		err = json.Unmarshal(jsonBytes, &userDevice)
		if err == nil {
			return userDevice, nil
		}
		logger.Errorf("UserInfoRes反序列化失败:%v", err.Error())
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		logger.Debugf("UserInfoRes缓存不存在:%v", err.Error())
	}

	userDevice, err = ds.dd.UserDeviceGetByDeviceUUID(ctx, uuid)
	if err != nil {
		return userDevice, err
	}
	jsonBytes, err = json.Marshal(userDevice)
	if err != nil {
		logger.Errorf("UserInfoRes序列化失败:%v", err.Error())
		return userDevice, err
	}
	// 存储到redis中 10分钟过期
	err = ds.rc.Set(ctx, rdsKey, jsonBytes, time.Hour*24).Err()
	if err != nil {
		logger.Errorf("UserInfoRes存储Cache失败:%v", err.Error())
		return userDevice, nil
	}

	return userDevice, err
}

func (u *deviceService) UserDeviceUpdateByDeviceUUID(ctx *gin.Context, ud entity.UserDevice) error {

	err := u.dd.UserDeviceUpdateByUUID(ctx, ud)
	if err != nil {
		return err
	}
	jsonBytes, err := json.Marshal(ud)
	if err != nil {
		logger.Errorf("UserInfoRes序列化失败:%v", err.Error())
		return err
	}
	rdsKey := consts.UserDeviceInfoPrefix + ud.UUID
	// 存储到redis中 48小时过期
	err = u.rc.Set(ctx, rdsKey, jsonBytes, time.Hour*48).Err()
	if err != nil {
		logger.Errorf("UserInfoRes存储Cache失败:%v", err.Error())
		return nil
	}

	return err
}
