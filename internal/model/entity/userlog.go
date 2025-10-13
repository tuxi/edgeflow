package entity

import "edgeflow/utils"

type UserLog struct {
	Id int64 `gorm:"column:id;primary_key;" json:"id"`
	// 可以为空，当用户登陆失败时也记录
	UserId    int64          `gorm:"column:user_id" json:"user_id"`
	UserIP    string         `gorm:"column:user_ip" json:"user_ip"`
	Business  string         `gorm:"column:business" json:"business"`
	Operation string         `gorm:"column:operation" json:"operation"`
	CreatedAt utils.JsonTime `gorm:"column:created_at" json:"created_at"`
	UpdatedAt utils.JsonTime `gorm:"column:updated_at" json:"updated_at"`
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
	DeviceModelDesc string `gorm:"device_model_desc" json:"device_model_desc"`
	// 设备型号
	DeviceModel string `gorm:"device_model" json:"device_model"`
	Radio       string `gorm:"radio" json:"radio"`
	// 运营商
	Carrier string `gorm:"carrier" json:"carrier"`
	// 是否是wifi
	IsWifi bool `gorm:"is_wifi" json:"is_wifi"`
	// 使用的代理
	Proxy string `gorm:"proxy" json:"proxy"`
	// 使用的语言
	LanguageId string `gorm:"language_id" json:"language_id"`
}

func (UserLog) TableName() string {
	return "userlog"
}
