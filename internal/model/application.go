package model

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	applicationv1 "go.admiral.io/sdk/proto/admiral/application/v1"
	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
)

type Application struct {
	Id          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name        string    `gorm:"uniqueIndex;not null"`
	Description string    `gorm:"type:text"`
	Labels      Labels    `gorm:"type:jsonb;default:'{}'"`
	CreatedBy   string    `gorm:"not null"`
	UpdatedBy   string    `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (app *Application) ToProto() *applicationv1.Application {
	return &applicationv1.Application{
		Id:          app.Id.String(),
		Name:        app.Name,
		Description: app.Description,
		Labels:      map[string]string(app.Labels),
		CreatedBy: &commonv1.ActorRef{
			Id: app.CreatedBy,
		},
		UpdatedBy: &commonv1.ActorRef{
			Id: app.UpdatedBy,
		},
		CreatedAt: timestamppb.New(app.CreatedAt),
		UpdatedAt: timestamppb.New(app.UpdatedAt),
	}
}
