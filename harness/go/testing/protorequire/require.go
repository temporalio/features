package protorequire

import (
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/temporalio/features/harness/go/testing/protoassert"
)

type helper interface {
	Helper()
}

type ProtoAssertions struct {
	t require.TestingT
}

func New(t require.TestingT) ProtoAssertions {
	return ProtoAssertions{t}
}

func ProtoEqual(t require.TestingT, a proto.Message, b proto.Message) {
	if th, ok := t.(helper); ok {
		th.Helper()
	}
	if !protoassert.ProtoEqual(t, a, b) {
		t.FailNow()
	}
}

func (x ProtoAssertions) ProtoEqual(a proto.Message, b proto.Message) {
	if th, ok := x.t.(helper); ok {
		th.Helper()
	}
	if !protoassert.ProtoEqual(x.t, a, b) {
		x.t.FailNow()
	}
}
