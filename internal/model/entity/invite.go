package entity

import "edgeflow/utils"

type Invite struct {
	Id           int64          `gorm:"column:id;primary_key;" json:"id"`
	UserId       int64          `gorm:"column:user_id" json:"user_id"`
	InviteCode   string         `gorm:"column:invite_code" json:"invite_code"`
	InviteNumber int            `gorm:"column:invite_number" json:"invite_number"`
	CreatedAt    utils.JsonTime `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    utils.JsonTime `gorm:"column:updated_at" json:"updated_at"`
}

func (Invite) TableName() string {
	return "public.invite"
}
