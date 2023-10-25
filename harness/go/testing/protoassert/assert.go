// Assert wraps testify's require package with useful helpers
package protoassert

import (
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type helper interface {
	Helper()
}

type ProtoAssertions struct {
	t assert.TestingT
}

func New(t assert.TestingT) ProtoAssertions {
	return ProtoAssertions{t}
}

// ProtoEqual compares two proto.Message objects for equality
func ProtoEqual(t assert.TestingT, a proto.Message, b proto.Message) bool {
	if th, ok := t.(helper); ok {
		th.Helper()
	}
	if diff := cmp.Diff(a, b, protocmp.Transform()); diff != "" {
		return assert.Fail(t, "Proto mismatch (-want +got):\n", diff)
	}
	return true
}

// ProtoSliceEqual compares elements in a slice of proto.Message.
// This is not a method on the suite type because methods cannot have
// generic parameters and slice casting (say from []historyEvent) to
// []proto.Message is impossible
func ProtoSliceEqual[T proto.Message](t assert.TestingT, a []T, b []T) bool {
	if th, ok := t.(helper); ok {
		th.Helper()
	}
	for i := 0; i < len(a); i++ {
		if diff := cmp.Diff(a[i], b[i], protocmp.Transform()); diff != "" {
			return assert.Fail(t, "Proto mismatch at index %d (-want +got):\n%v", i, diff)
		}
	}

	return true
}

func (x ProtoAssertions) ProtoEqual(a proto.Message, b proto.Message) bool {
	if th, ok := x.t.(helper); ok {
		th.Helper()
	}
	return ProtoEqual(x.t, a, b)
}
