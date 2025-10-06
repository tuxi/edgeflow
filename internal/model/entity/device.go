package entity

import (
	"crypto/rand"
	"edgeflow/utils"
	"edgeflow/utils/security"
	"encoding/base64"
	"encoding/json"
	"gorm.io/plugin/soft_delete"
)

type DeviceToken struct {
	Id          int64                 `gorm:"column:id;primary_key;" json:"id"`
	DeviceToken string                `gorm:"column:device_token;not null" json:"device_token"` // 设备标识符
	DeviceUUID  string                `gorm:"column:device_uuid;not null" json:"device_uuid"`   // 设备的UUID
	Platform    string                `gorm:"platform;not null" json:"platform"`                // 设备平台
	CreatedAt   utils.JsonTime        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   utils.JsonTime        `gorm:"column:deleted_at" json:"deleted_at" `
	IsDel       soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
}

func (DeviceToken) TableName() string {
	return "public.devicetoken"
}

// 通过UserDevice表关联User表和DeviceToken表，而不让User和DeviceToken直接关联，要实现无用户登陆的推送
type UserDevice struct {
	Id     int64  `gorm:"column:id;primary_key;" json:"id"`
	UserId int64  `gorm:"column:user_id;not null" json:"user_id"` // User的外键
	UUID   string `gorm:"uuid;not null;unique" json:"uuid"`
	//  用于在端对端加密的加盐base64字符串
	AuthKey string `gorm:"column:auth_key;not null" json:"auth_key"`
	// 设备的公钥，用来配合服务端的私钥加密数据的
	ClientPublicKey string `gorm:"column:client_public_key;not null" json:"client_public_key"`
	// 服务端生成的公钥和私钥，定期更新
	ServicePrivateKey string `gorm:"column:service_private_key;not null" json:"service_private_key"`
	ServicePublicKey  string `gorm:"column:service_public_key;not null" json:"service_public_key"`

	DeviceTokenId *int64                `gorm:"column:device_token_id;" json:"device_token_id"` // DeviceToken的外键
	DeviceToken   DeviceToken           `gorm:"foreignKey:device_token_id"`
	DeviceId      *int64                `gorm:"column:device_id;not null" json:"device_id"`
	Device        Device                `gorm:"foreignKey:device_id"`
	CreatedAt     utils.JsonTime        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt     utils.JsonTime        `gorm:"column:deleted_at" json:"deleted_at" `
	IsDel         soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
}

func (UserDevice) TableName() string {
	return "public.userdevice"
}

type Device struct {
	Id        int64                 `gorm:"column:id;primary_key;" json:"id"`
	UUID      string                `gorm:"uuid;not null;unique" json:"uuid"`
	CreatedAt utils.JsonTime        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`
	IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
	ClientIP  string                `gorm:"column:client_ip" json:"client_ip"`
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
	// 设备序列号，仅mac有
	SerialNumber string `gorm:"serial_number" json:"serial_number"`
	// 硬件id
	PlatformUUID string `gorm:"platform_uuid" json:"platform_uuid"`
}

func (Device) TableName() string {
	return "public.device"
}

// 客户端版本
type Version struct {
	Id                  int64                 `gorm:"column:id;primary_key;" json:"id"`                   // 主键id
	VersionNumber       string                `gorm:"column:version_number" json:"version_number"`        // 版本号                                // 版本号
	IsMandatory         bool                  `gorm:"column:is_mandatory" json:"is_mandatory"`            // 是否强制更新。
	UpdateInfo          string                `gorm:"column:update_info" json:"update_info"`              // 更新内容。
	DownloadURL         string                `gorm:"column:download_url" json:"download_url"`            // 下载地址
	Platform            string                `gorm:"platform" json:"platform"`                           // 设备平台
	MinSupportedVersion string                `gorm:"min_supported_version" json:"min_supported_version"` // 最小支持的版本
	CreatedAt           utils.JsonTime        `gorm:"column:created_at" json:"created_at"`                // 创建时间
	UpdatedAt           utils.JsonTime        `gorm:"column:updated_at" json:"updated_at"`                // 更新时间
	ReleasedAt          utils.JsonTime        `gorm:"column:released_at" json:"released_at"`              // 发布时间
	DeletedAt           utils.JsonTime        `gorm:"column:deleted_at" json:"deleted_at" `               // 删除时间
	IsDel               soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`           // 是否删除，软删除
	Status              string                `gorm:"status" json:"status"`                               // 审核状态，默认为审核中，'审查中', '已发布', '拒绝'
}

func (Version) TableName() string {
	return "public.version"
}

func (ud *UserDevice) UserDeviceEncrypt(payload interface{}) (s, n, content string, er error) {
	// salt每次改变
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		return
	}

	servicePrivateKey, err := base64.StdEncoding.DecodeString(ud.ServicePrivateKey)
	clientPublicKey, err := base64.StdEncoding.DecodeString(ud.ClientPublicKey)
	authKey, err := base64.StdEncoding.DecodeString(ud.AuthKey)
	if err != nil {
		return
	}
	chaCha, err := security.NewChaChaPoly(servicePrivateKey, clientPublicKey, salt, authKey, nil)
	if err != nil {
		return
	}
	ciphertext, err := chaCha.Encrypt(payloadData)
	if err != nil {
		return
	}
	s = base64.StdEncoding.EncodeToString(salt)
	n = base64.StdEncoding.EncodeToString(chaCha.Nonce)
	content = base64.StdEncoding.EncodeToString(ciphertext)
	return
}
func (ud *UserDevice) UserDeviceDecrypt(s, n, content string) (payloadData []byte, err error) {
	servicePrivateKey, err := base64.StdEncoding.DecodeString(ud.ServicePrivateKey)
	clientPublicKey, err := base64.StdEncoding.DecodeString(ud.ClientPublicKey)
	authKey, err := base64.StdEncoding.DecodeString(ud.AuthKey)
	salt, err := base64.StdEncoding.DecodeString(s)
	result, err := base64.StdEncoding.DecodeString(content)
	nonce, err := base64.StdEncoding.DecodeString(n)
	if err != nil {
		return
	}
	chaCha, err := security.NewChaChaPoly(servicePrivateKey, clientPublicKey, salt, authKey, nonce)
	if err != nil {
		return
	}
	payloadData, err = chaCha.Decrypt(result)
	return
}
