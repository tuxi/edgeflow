package query

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
)

var _ dao.DeviceDao = (*deviceDao)(nil)

type deviceDao struct {
	ds *gorm.DB
}

func NewDeviceDao(ds *gorm.DB) *deviceDao {
	return &deviceDao{
		ds: ds,
	}
}

func (u *deviceDao) UserDeviceTokenListGetByUserId(ctx context.Context, userId int64) ([]entity.DeviceToken, error) {
	var userDevices []entity.UserDevice
	err := u.ds.WithContext(ctx).Where("user_id = ?", userId).Preload("DeviceToken").Find(&userDevices).Error
	if err != nil {
		return nil, err
	}

	var deviceTokens []entity.DeviceToken
	for _, ud := range userDevices {
		deviceTokens = append(deviceTokens, ud.DeviceToken)
	}
	return deviceTokens, nil
}

func (u *deviceDao) UserDeviceTokenGetList(ctx context.Context) ([]entity.DeviceToken, error) {
	var tokens []entity.DeviceToken
	err := u.ds.WithContext(ctx).Model(entity.DeviceToken{}).Find(&tokens).Error
	return tokens, err
}

func (u *deviceDao) UserDeviceTokenCreateNew(ctx context.Context, deviceToken entity.DeviceToken) error {
	err := u.ds.WithContext(ctx).Create(&deviceToken).Error
	return err
}

func (u *deviceDao) UserDeviceTokenUpdateByDeviceUUID(ctx context.Context, deviceUUID, deviceToken string) error {
	err := u.ds.WithContext(ctx).
		Model(&entity.DeviceToken{}).
		Where("device_uuid = ?", deviceUUID).
		UpdateColumn("device_token", deviceToken).
		Error
	return err
}

func (u *deviceDao) UserDeviceTokenGetByDeviceUUID(ctx context.Context, deviceUUID string) (entity.DeviceToken, error) {
	var token entity.DeviceToken
	err := u.ds.WithContext(ctx).Where("device_uuid = ?", deviceUUID).First(&token).Error
	return token, err
}

func (u *deviceDao) UserDeviceGetByDeviceTokenId(ctx context.Context, userId, deviceTokenId int64) (entity.UserDevice, error) {
	var userDevice entity.UserDevice
	err := u.ds.WithContext(ctx).Where("user_id = ? AND device_token_id = ?", userId, deviceTokenId).First(&userDevice).Error
	return userDevice, err
}

func (u *deviceDao) UserDeviceGetByDeviceId(ctx context.Context, userId, deviceId int64) (entity.UserDevice, error) {
	var userDevice entity.UserDevice
	err := u.ds.WithContext(ctx).Where("user_id = ? AND device_id = ?", userId, deviceId).First(&userDevice).Error
	return userDevice, err
}

func (u *deviceDao) UserDeviceGetByDeviceUUID(ctx context.Context, uuid string) (entity.UserDevice, error) {
	var userDevice entity.UserDevice
	err := u.ds.WithContext(ctx).Where("uuid = ?", uuid).First(&userDevice).Error
	return userDevice, err
}

func (u *deviceDao) UserDeviceCreateNew(ctx context.Context, userDevice entity.UserDevice) error {
	return u.ds.WithContext(ctx).Create(&userDevice).Error
}

// 根据userId和device_key获取device token
func (u *deviceDao) UserDeviceTokenGetByUserId(ctx context.Context, userId int64, deviceKey string) (entity.DeviceToken, error) {
	var token entity.DeviceToken
	err := u.ds.WithContext(ctx).Model(entity.DeviceToken{}).Where("user_id = ?", userId).Where("device_key = ?", deviceKey).Find(&token).Error
	return token, err
}

func (u *deviceDao) DeviceGetByUserId(ctx context.Context, userId int64) (entity.Device, error) {
	var device entity.Device
	err := u.ds.WithContext(ctx).Where("user_id = ?", userId).Preload("Device").First(&device).Error
	if err != nil {
		return device, err
	}
	return device, nil
}

func (u *deviceDao) DeviceCreateNew(ctx context.Context, device entity.Device) error {
	return u.ds.WithContext(ctx).Create(&device).Error
}

func (u *deviceDao) DeviceGetByUUID(ctx context.Context, uuid string) (entity.Device, error) {
	var device entity.Device
	err := u.ds.WithContext(ctx).Where("uuid = ?", uuid).First(&device).Error
	return device, err
}

func (u *deviceDao) UserDeviceUpdateByUUID(ctx context.Context, ud entity.UserDevice) error {
	err := u.ds.WithContext(ctx).Updates(&ud).Error
	return err
}

func (u *deviceDao) DeviceUpdateByUUID(ctx context.Context, device entity.Device) error {
	err := u.ds.WithContext(ctx).Updates(&device).Error
	return err
}
