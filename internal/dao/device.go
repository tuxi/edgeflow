package dao

import (
	"context"
	"edgeflow/internal/model/entity"
)

type DeviceDao interface {
	// 获取所有的device token
	UserDeviceTokenGetList(ctx context.Context) ([]entity.DeviceToken, error)
	// 根据userId获取所有device token
	UserDeviceTokenListGetByUserId(ctx context.Context, userId int64) ([]entity.DeviceToken, error)
	UserDeviceTokenGetByDeviceUUID(ctx context.Context, deviceKey string) (entity.DeviceToken, error)
	// 根据userId和uuid获取device token
	UserDeviceTokenGetByUserId(ctx context.Context, userId int64, deviceKey string) (entity.DeviceToken, error)
	// 创建DeviceToken
	UserDeviceTokenCreateNew(ctx context.Context, deviceToken entity.DeviceToken) error
	// 根据用户id和deviceUUID更新deviceToken
	UserDeviceTokenUpdateByDeviceUUID(ctx context.Context, deviceUUID, deviceToken string) error
	UserDeviceUpdateByUUID(ctx context.Context, ud entity.UserDevice) error
	// 根据用户id和设备id查找
	UserDeviceGetByDeviceTokenId(ctx context.Context, userId, deviceTokenId int64) (entity.UserDevice, error)
	UserDeviceGetByDeviceId(ctx context.Context, userId, deviceId int64) (entity.UserDevice, error)
	UserDeviceGetByDeviceUUID(ctx context.Context, uuid string) (entity.UserDevice, error)
	// 创建UserDevice
	UserDeviceCreateNew(ctx context.Context, userDevice entity.UserDevice) error
	// 创建设备
	DeviceCreateNew(ctx context.Context, device entity.Device) error
	// 根据用户id获取Device
	DeviceGetByUserId(ctx context.Context, userId int64) (entity.Device, error)
	DeviceGetByUUID(ctx context.Context, uuid string) (entity.Device, error)
	DeviceUpdateByUUID(ctx context.Context, device entity.Device) error
}
