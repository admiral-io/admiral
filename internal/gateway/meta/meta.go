package meta

import (
	"fmt"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	methodDescriptors map[string]*desc.MethodDescriptor
)

func APIBody(body interface{}) (*anypb.Any, error) {
	m, ok := body.(proto.Message)
	if !ok {
		// body is not the model/value we want to process
		return nil, nil
	}

	// Deep copy before field redaction so we do not unintentionally remove fields
	// from the original object that were passed by reference
	m = proto.Clone(m)
	return anypb.New(m)
}

func GenerateGRPCMetadata(server *grpc.Server) error {
	serviceDescriptors, err := grpcreflect.LoadServiceDescriptors(server)
	if err != nil {
		return err
	}

	mds := make(map[string]*desc.MethodDescriptor)
	for _, sd := range serviceDescriptors {
		for _, md := range sd.GetMethods() {
			methodName := fmt.Sprintf("/%s/%s", sd.GetFullyQualifiedName(), md.GetName())
			mds[methodName] = md
		}
	}

	methodDescriptors = mds
	return nil
}
