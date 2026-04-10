package model

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	userv1 "go.admiral.io/sdk/proto/admiral/user/v1"
)

type UserInfo struct {
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	PictureUrl    string
}

type User struct {
	Id              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProviderSubject string
	Email           string
	EmailVerified   bool
	Name            string
	GivenName       string
	FamilyName      string
	PictureUrl      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (u *User) ToProto() *userv1.User {
	proto := &userv1.User{
		Id:            u.Id.String(),
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		CreatedAt:     timestamppb.New(u.CreatedAt),
		UpdatedAt:     timestamppb.New(u.UpdatedAt),
	}

	if u.Name != "" {
		proto.DisplayName = &u.Name
	}

	if u.GivenName != "" {
		proto.GivenName = &u.GivenName
	}

	if u.FamilyName != "" {
		proto.FamilyName = &u.FamilyName
	}

	if u.PictureUrl != "" {
		proto.AvatarUrl = &u.PictureUrl
	}

	return proto
}
