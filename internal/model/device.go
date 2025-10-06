package model

import "edgeflow/utils"

type DeviceTokenReportReq struct {
	DeviceUUID  string `json:"device_uuid" validate:"required" label:"app内生成的唯一识别码"`
	DeviceToken string `json:"device_token" validate:"required" label:"开发商的设备token"`
	Platform    string `json:"platform" validate:"required" label:"区分平台，比如苹果、小米"`
}

type UserPushMessageReq struct {
	UserPushMessageGlobalReq
	DeviceUUID string `json:"device_uuid"  validate:"required" label:"关联device的uuid"` // 就是客户端存储的uuid
}

type DeviceTokenOne struct {
	DeviceUUID string         `gorm:"column:device_uuid;not null" json:"device_uuid"` // 设备的UUID
	Platform   string         `gorm:"platform;not null" json:"platform"`              // 设备平台
	CreatedAt  utils.JsonTime `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  utils.JsonTime `gorm:"column:updated_at" json:"updated_at"`
}

type DeviceTokenListRes struct {
	Devices []DeviceTokenOne `json:"devices"`
}

type UserPushMessageGlobalReq struct {
	Category  string                 `json:"category"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Sound     string                 `json:"sound"`
	ExtParams map[string]interface{} `json:"ext_params"`
}

// 设备上报
type DeviceReportReq struct {
	UUID string `json:"uuid" validate:"required"`
	// 屏幕高度
	ScreenHeight int64 `json:"screen_height"`
	// 屏幕宽度
	ScreenWidth int64 `json:"screen_width"`
	// 系统名称
	Os string `json:"os"`
	// 系统版本号
	OsVersion string `json:"os_version"`
	// app编译版本号
	AppBuildNumber string `json:"app_build_number"`
	// App 版本号
	AppVersion string `json:"app_version"`
	// 平台 appstore或者小米商店
	AppPlatform string `json:"app_platform"`
	// app包名
	AppPackageId string `json:"app_package_id"`
	// 设备品牌
	DeviceBrand string `json:"device_brand"`
	// 设备型号
	DeviceModelDesc string `json:"device_model_desc"`
	// 设备型号
	DeviceModel string `json:"device_model"`
	Radio       string `json:"radio"`
	// 运营商
	Carrier string `json:"carrier"`
	// 是否是wifi
	IsWifi bool `json:"is_wifi"`
	// 使用的代理
	Proxy string `json:"proxy"`
	// 使用的语言
	LanguageId string `json:"language_id"`
	// 设备序列号，仅mac有
	SerialNumber string `json:"serial_number"`
	// 硬件id
	PlatformUUID string `json:"platform_uuid"`
}

type AppVersionLatestReq struct {
	Platform string `json:"platform" validate:"required"` // 平台
}

type AppVersionCurrentReq struct {
	Platform      string `json:"platform" validate:"required"`       // 平台
	VersionNumber string `json:"version_number" validate:"required"` // 版本号
}

type AppVersionOne struct {
	VersionNumber       string         `gorm:"column:version_number" json:"version_number"`        // 版本号                                // 版本号
	IsMandatory         bool           `gorm:"column:is_mandatory" json:"is_mandatory"`            // 是否强制更新。
	UpdateInfo          string         `gorm:"column:update_info" json:"update_info"`              // 更新内容。
	DownloadURL         string         `gorm:"column:download_url" json:"download_url"`            // 下载地址
	Platform            string         `gorm:"platform" json:"platform"`                           // 设备平台
	MinSupportedVersion string         `gorm:"min_supported_version" json:"min_supported_version"` // 最小支持的版本
	ReleasedAt          utils.JsonTime `gorm:"column:released_at" json:"released_at"`              // 发布时间
}

type AppVersionCreateReq struct {
	VersionNumber       string `json:"version_number" validate:"required"` // 版本号
	Platform            string `json:"platform" validate:"required"`       // 设备平台
	IsMandatory         bool   `json:"is_mandatory"`                       // 是否强制更新。
	UpdateInfo          string `json:"update_info"`                        // 更新内容。
	DownloadURL         string `json:"download_url"`                       // 下载地址
	MinSupportedVersion string `json:"min_supported_version"`              // 最小支持的版本
}

type AppVersionReleaseReq struct {
	VersionId string `json:"version_id" validate:"required"` // 要发布的版本id
}

// 单个聊天消息的响应体
type AppVersionOneRes struct {
	Id                  string         `json:"version_id"`
	VersionNumber       string         `json:"version_number"`        // 版本号
	Platform            string         `json:"platform"`              // 设备平台
	IsMandatory         bool           `json:"is_mandatory"`          // 是否强制更新。
	UpdateInfo          string         `json:"update_info"`           // 更新内容。
	DownloadURL         string         `json:"download_url"`          // 下载地址
	MinSupportedVersion string         `json:"min_supported_version"` // 最小支持的版本
	CreatedAt           utils.JsonTime `json:"created_at"`            // 创建时间
	UpdatedAt           utils.JsonTime `json:"updated_at"`            // 更新时间
	ReleasedAt          utils.JsonTime `json:"released_at"`           // 发布时间
	Status              string         `json:"status"`                // 状态
}

type VersionListGetRes struct {
	VersionList []AppVersionOneRes `json:"version_list"`
}

type VersionStatusRes struct {
	Status string `json:"status"`
}

// 使用ChaChaPoly加密后的data
type ChaChaPolyEncryptData struct {
	Payload string `json:"payload" validate:"required"`
	S       string `json:"s" validate:"required"` // salt
	N       string `json:"n" validate:"required"` // Nonce
}
