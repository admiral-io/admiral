package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
	variablev1 "go.admiral.io/sdk/proto/admiral/variable/v1"
)

const (
	VariableTypeString  = "STRING"
	VariableTypeNumber  = "NUMBER"
	VariableTypeBoolean = "BOOLEAN"
	VariableTypeComplex = "COMPLEX"
)

const (
	VariableSourceUser           = "USER"
	VariableSourceInfrastructure = "INFRASTRUCTURE"
)

var variableTypeToProto = map[string]variablev1.VariableType{
	VariableTypeString:  variablev1.VariableType_VARIABLE_TYPE_STRING,
	VariableTypeNumber:  variablev1.VariableType_VARIABLE_TYPE_NUMBER,
	VariableTypeBoolean: variablev1.VariableType_VARIABLE_TYPE_BOOLEAN,
	VariableTypeComplex: variablev1.VariableType_VARIABLE_TYPE_COMPLEX,
}

var variableTypeFromProto = map[variablev1.VariableType]string{
	variablev1.VariableType_VARIABLE_TYPE_STRING:  VariableTypeString,
	variablev1.VariableType_VARIABLE_TYPE_NUMBER:  VariableTypeNumber,
	variablev1.VariableType_VARIABLE_TYPE_BOOLEAN: VariableTypeBoolean,
	variablev1.VariableType_VARIABLE_TYPE_COMPLEX: VariableTypeComplex,
}

var variableSourceToProto = map[string]variablev1.VariableSource{
	VariableSourceUser:           variablev1.VariableSource_VARIABLE_SOURCE_USER,
	VariableSourceInfrastructure: variablev1.VariableSource_VARIABLE_SOURCE_INFRASTRUCTURE,
}

func VariableTypeFromProto(t variablev1.VariableType) string {
	return variableTypeFromProto[t]
}

type Variable struct {
	Id             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Key            string    `gorm:"type:varchar(63);not null"`
	Value          string    `gorm:"type:text;not null;default:''"`
	Sensitive      bool      `gorm:"not null;default:false"`
	Type           string    `gorm:"type:text;not null;default:'STRING'"`
	Source         string    `gorm:"type:text;not null;default:'USER'"`
	Description    string    `gorm:"type:varchar(1024);not null;default:''"`
	ApplicationId  uuid.UUID `gorm:"type:uuid;not null"`
	EnvironmentId  uuid.UUID `gorm:"type:uuid;not null"`
	CreatedBy      string    `gorm:"type:text;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedByName  string `gorm:"->;column:created_by_name"`
	CreatedByEmail string `gorm:"->;column:created_by_email"`
}

func (v *Variable) ToProto() *variablev1.Variable {
	appID := v.ApplicationId.String()
	envID := v.EnvironmentId.String()
	out := &variablev1.Variable{
		Id:            v.Id.String(),
		Key:           v.Key,
		Value:         v.Value,
		Sensitive:     v.Sensitive,
		Type:          variableTypeToProto[v.Type],
		Source:        variableSourceToProto[v.Source],
		Description:   v.Description,
		ApplicationId: &appID,
		EnvironmentId: &envID,
		CreatedBy:     &commonv1.ActorRef{Id: v.CreatedBy, DisplayName: v.CreatedByName, Email: v.CreatedByEmail},
		CreatedAt:     timestamppb.New(v.CreatedAt),
		UpdatedAt:     timestamppb.New(v.UpdatedAt),
	}

	if v.Sensitive {
		out.Value = ""
	}

	return out
}

func (v *Variable) Validate() error {
	if v.Key == "" {
		return fmt.Errorf("key is required")
	}
	if len(v.Key) > 63 {
		return fmt.Errorf("key must be 63 characters or less")
	}
	if err := ValidateVariableValue(v.Type, v.Value); err != nil {
		return err
	}
	switch v.Source {
	case VariableSourceUser, VariableSourceInfrastructure:
	case "":
		return fmt.Errorf("source is required")
	default:
		return fmt.Errorf("invalid source: %s", v.Source)
	}
	if v.ApplicationId == uuid.Nil {
		return fmt.Errorf("application_id is required")
	}
	if v.EnvironmentId == uuid.Nil {
		return fmt.Errorf("environment_id is required")
	}
	if v.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	return nil
}

func ValidateVariableValue(varType, value string) error {
	switch varType {
	case VariableTypeString:
		// No constraint on string values.
	case VariableTypeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("value must be a valid number for type NUMBER")
		}
	case VariableTypeBoolean:
		if value != "true" && value != "false" {
			return fmt.Errorf("value must be \"true\" or \"false\" for type BOOLEAN")
		}
	case VariableTypeComplex:
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("value must be valid JSON for type COMPLEX")
		}
	case "":
		return fmt.Errorf("variable type is required")
	default:
		return fmt.Errorf("unsupported variable type: %s", varType)
	}
	return nil
}

func VariablesFromEngineOutputs(
	outputs map[string]*runnerv1.EngineOutput,
	componentSlug string,
	appID, envID uuid.UUID,
	createdBy string,
) []Variable {
	vars := make([]Variable, 0, len(outputs))
	for name, out := range outputs {
		vars = append(vars, Variable{
			Key:           componentSlug + "." + name,
			Value:         out.GetValue(),
			Sensitive:     out.GetSensitive(),
			Type:          tfTypeToVariableType(out.GetType()),
			Source:        VariableSourceInfrastructure,
			ApplicationId: appID,
			EnvironmentId: envID,
			CreatedBy:     createdBy,
		})
	}
	return vars
}

func tfTypeToVariableType(tfType string) string {
	t := strings.ToLower(strings.TrimSpace(tfType))
	switch t {
	case "string":
		return VariableTypeString
	case "number":
		return VariableTypeNumber
	case "bool":
		return VariableTypeBoolean
	default:
		return VariableTypeComplex
	}
}
