package meta

import (
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	mu              sync.RWMutex
	methodOptions   map[string]*descriptorpb.MethodOptions
	methodsResolved bool
)

func ResolveMethodOptions(server *grpc.Server) error {
	mu.Lock()
	defer mu.Unlock()

	opts := make(map[string]*descriptorpb.MethodOptions)

	for name := range server.GetServiceInfo() {
		sd, err := resolveServiceDescriptor(protoreflect.FullName(name))
		if err != nil {
			return fmt.Errorf("failed to resolve service %s: %w", name, err)
		}

		methods := sd.Methods()
		for i := range methods.Len() {
			md := methods.Get(i)
			fullMethod := fmt.Sprintf("/%s/%s", sd.FullName(), md.Name())
			opts[fullMethod] = protodesc.ToMethodDescriptorProto(md).GetOptions()
		}
	}

	methodOptions = opts
	methodsResolved = true
	return nil
}

func GetMethodOptions(fullMethod string) *descriptorpb.MethodOptions {
	mu.RLock()
	defer mu.RUnlock()
	return methodOptions[fullMethod]
}

func MethodsResolved() bool {
	mu.RLock()
	defer mu.RUnlock()
	return methodsResolved
}

func resolveServiceDescriptor(name protoreflect.FullName) (protoreflect.ServiceDescriptor, error) {
	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}

	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a service descriptor", name)
	}

	return sd, nil
}

func APIBody(body any) (*anypb.Any, error) {
	m, ok := body.(proto.Message)
	if !ok {
		return nil, nil
	}

	m = proto.Clone(m)
	return anypb.New(m)
}
