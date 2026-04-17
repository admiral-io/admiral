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

// Variable type constants match the CHECK constraint in the migration.
const (
	VariableTypeString  = "STRING"
	VariableTypeNumber  = "NUMBER"
	VariableTypeBoolean = "BOOLEAN"
	VariableTypeComplex = "COMPLEX"
)

// Variable source constants.
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
	Id            uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Key           string     `gorm:"type:varchar(63);not null"`
	Value         string     `gorm:"type:text;not null;default:''"`
	Sensitive     bool       `gorm:"not null;default:false"`
	Type          string     `gorm:"type:text;not null;default:'STRING'"`
	Source        string     `gorm:"type:text;not null;default:'USER'"`
	Description   string     `gorm:"type:varchar(1024);not null;default:''"`
	ApplicationId *uuid.UUID `gorm:"type:uuid"`
	EnvironmentId *uuid.UUID `gorm:"type:uuid"`
	CreatedBy     string     `gorm:"type:text;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (v *Variable) Validate() error {
	switch v.Type {
	case VariableTypeString:
		// No constraint on string values.
	case VariableTypeNumber:
		if _, err := strconv.ParseFloat(v.Value, 64); err != nil {
			return fmt.Errorf("value must be a valid number for type NUMBER")
		}
	case VariableTypeBoolean:
		if v.Value != "true" && v.Value != "false" {
			return fmt.Errorf("value must be \"true\" or \"false\" for type BOOLEAN")
		}
	case VariableTypeComplex:
		if !json.Valid([]byte(v.Value)) {
			return fmt.Errorf("value must be valid JSON for type COMPLEX")
		}
	case "":
		return fmt.Errorf("variable type is required")
	default:
		return fmt.Errorf("unsupported variable type: %s", v.Type)
	}

	if v.EnvironmentId != nil && v.ApplicationId == nil {
		return fmt.Errorf("environment_id requires application_id")
	}

	return nil
}

func (v *Variable) ToProto() *variablev1.Variable {
	out := &variablev1.Variable{
		Id:          v.Id.String(),
		Key:         v.Key,
		Value:       v.Value,
		Sensitive:   v.Sensitive,
		Type:        variableTypeToProto[v.Type],
		Source:      variableSourceToProto[v.Source],
		Description: v.Description,
		CreatedBy:   &commonv1.ActorRef{Id: v.CreatedBy},
		CreatedAt:   timestamppb.New(v.CreatedAt),
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
	}

	if v.ApplicationId != nil {
		s := v.ApplicationId.String()
		out.ApplicationId = &s
	}
	if v.EnvironmentId != nil {
		s := v.EnvironmentId.String()
		out.EnvironmentId = &s
	}
	if v.Sensitive {
		out.Value = ""
	}

	return out
}

func VariablesFromTerraformOutputs(
	outputs map[string]*runnerv1.TerraformOutput,
	componentName string,
	appID, envID uuid.UUID,
	createdBy string,
) []Variable {
	vars := make([]Variable, 0, len(outputs))
	for name, out := range outputs {
		vars = append(vars, Variable{
			Key:           componentName + "." + name,
			Value:         out.GetValue(),
			Sensitive:     out.GetSensitive(),
			Type:          tfTypeToVariableType(out.GetType()),
			Source:        VariableSourceInfrastructure,
			ApplicationId: &appID,
			EnvironmentId: &envID,
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
