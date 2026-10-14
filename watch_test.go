package cz_test

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"github.com/lesomnus/cz"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestWatchFilter(t *testing.T) {
	manifestType, eventType := watchDescriptors(t)
	event := cz.Ident("event")
	value := event.Select("value")
	const artifactType = "application/vnd.dev.sigstore.bundle.v0.3+json"
	const method = "/cr.index.Manifest/Put"
	filter := cz.And(
		cz.Has(value),
		cz.Eq(cz.Type(value), cz.Ident("app.Manifest")),
		cz.Eq(event.Select("kind"), cz.Ident("payday.CHANGE_KIND_CREATED")),
		cz.Eq(value.Select("artifact_type"), cz.String(artifactType)),
		cz.Eq(event.Select("method"), cz.String(method)),
	)
	env, err := cel.NewEnv(
		cel.Types(dynamicpb.NewMessage(manifestType), dynamicpb.NewMessage(eventType)),
		cel.Variable("event", cel.ObjectType("payday.WatchEvent")),
		// payday exposes short protobuf-style enum names in its CEL environment.
		cel.Constant("payday.CHANGE_KIND_CREATED", cel.IntType, types.Int(1)),
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := cz.Render(filter)
	if err != nil {
		t.Fatal(err)
	}
	compiled, issues := env.Compile(source)
	if issues.Err() != nil {
		t.Fatal(issues.Err())
	}
	program, err := env.Program(compiled)
	if err != nil {
		t.Fatal(err)
	}
	manifest := func(artifact string) proto.Message {
		msg := dynamicpb.NewMessage(manifestType)
		msg.Set(manifestType.Fields().ByName("artifact_type"), protoreflect.ValueOfString(artifact))
		return msg
	}
	tests := []struct {
		name   string
		value  proto.Message
		kind   protoreflect.EnumNumber
		method string
		want   types.Bool
	}{
		{"match", manifest(artifactType), 1, method, types.True},
		{"different artifact", manifest("image"), 1, method, types.False},
		{"empty artifact", manifest(""), 1, method, types.False},
		{"payload is data", manifest(`x" || true || "x`), 1, method, types.False},
		{"updated", manifest(artifactType), 2, method, types.False},
		{"other method", manifest(artifactType), 1, "/example/Other", types.False},
		{"no value", nil, 3, method, types.False},
		{"different Any type", wrapperspb.String(artifactType), 1, method, types.False},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := dynamicpb.NewMessage(eventType)
			msg.Set(eventType.Fields().ByName("kind"), protoreflect.ValueOfEnum(tt.kind))
			msg.Set(eventType.Fields().ByName("method"), protoreflect.ValueOfString(tt.method))
			if tt.value != nil {
				payload, err := anypb.New(tt.value)
				if err != nil {
					t.Fatal(err)
				}
				msg.Set(eventType.Fields().ByName("value"), protoreflect.ValueOfMessage(payload.ProtoReflect()))
			}
			got, _, err := program.Eval(map[string]any{"event": msg})
			if err != nil || got != tt.want {
				t.Fatalf("got (%v, %v), want %v", got, err, tt.want)
			}
		})
	}
}

// These small descriptors exercise the documented Watch example without
// importing payday or any application module into this general-purpose package.
func watchDescriptors(t *testing.T) (protoreflect.MessageDescriptor, protoreflect.MessageDescriptor) {
	t.Helper()
	field := func(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.FieldDescriptorProto {
		f := &descriptorpb.FieldDescriptorProto{
			Name: proto.String(name), Number: proto.Int32(number), Type: kind.Enum(),
			Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		if typeName != "" {
			f.TypeName = proto.String(typeName)
		}
		return f
	}
	manifest, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("test/manifest.proto"), Package: proto.String("app"), Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Manifest"),
			Field: []*descriptorpb.FieldDescriptorProto{
				field("artifact_type", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
			},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("test/watch.proto"), Package: proto.String("payday"), Syntax: proto.String("proto3"),
		Dependency: []string{"google/protobuf/any.proto"},
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("ChangeKind"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("CHANGE_KIND_UNSPECIFIED"), Number: proto.Int32(0)},
				{Name: proto.String("CHANGE_KIND_CREATED"), Number: proto.Int32(1)},
				{Name: proto.String("CHANGE_KIND_UPDATED"), Number: proto.Int32(2)},
				{Name: proto.String("CHANGE_KIND_DELETED"), Number: proto.Int32(3)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("WatchEvent"),
			Field: []*descriptorpb.FieldDescriptorProto{
				field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".google.protobuf.Any"),
				field("method", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
				field("kind", 5, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".payday.ChangeKind"),
			},
		}},
	}, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}
	manifestType := manifest.Messages().ByName("Manifest")
	// Generated messages register themselves globally. Dynamic test messages
	// need the same registration for Any.UnmarshalNew, which cel-go uses.
	if existing, err := protoregistry.GlobalTypes.FindMessageByName(manifestType.FullName()); err == nil {
		manifestType = existing.Descriptor()
	} else if err := protoregistry.GlobalTypes.RegisterMessage(dynamicpb.NewMessageType(manifestType)); err != nil {
		t.Fatal(err)
	}
	return manifestType, event.Messages().ByName("WatchEvent")
}
