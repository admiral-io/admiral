package log

import (
	"encoding/json"

	"go.uber.org/zap"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func ProtoField(key string, m proto.Message) zap.Field {
	b, err := protojson.Marshal(m)
	if err != nil {
		return zap.Any(key, m)
	}
	return zap.Any(key, json.RawMessage(b))
}

func NamedErrorField(key string, err error) zap.Field {
	s, ok := status.FromError(err)
	if !ok {
		return zap.NamedError(key, err)
	}
	return ProtoField(key, s.Proto())
}

func ErrorField(err error) zap.Field {
	return NamedErrorField("error", err)
}
