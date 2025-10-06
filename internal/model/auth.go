package model

type GetAnonymousAccessTokenReq struct {
	Sign string `json:"sign" validate:"required"`
}

type AnonymousAccessSign struct {
	BundleId           string `json:"bundle_id" validate:"required"`           // app包名
	UUID               string `json:"uuid" validate:"required"`                // 设备标识
	AuthKey            string `json:"auth_key" validate:"required"`            // 加盐
	Key                string `json:"key" validate:"required"`                 // p256的公钥
	HardwareIdentifier string `json:"hardware_identifier" validate:"required"` // 硬件标识符，只有macos 有
}
