// Code generated by protoc-gen-go. DO NOT EDIT.
// source: messages.proto

package messages

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type BinaryMessage struct {
	Data                 []byte   `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BinaryMessage) Reset()         { *m = BinaryMessage{} }
func (m *BinaryMessage) String() string { return proto.CompactTextString(m) }
func (*BinaryMessage) ProtoMessage()    {}
func (*BinaryMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{0}
}

func (m *BinaryMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BinaryMessage.Unmarshal(m, b)
}
func (m *BinaryMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BinaryMessage.Marshal(b, m, deterministic)
}
func (m *BinaryMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BinaryMessage.Merge(m, src)
}
func (m *BinaryMessage) XXX_Size() int {
	return xxx_messageInfo_BinaryMessage.Size(m)
}
func (m *BinaryMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_BinaryMessage.DiscardUnknown(m)
}

var xxx_messageInfo_BinaryMessage proto.InternalMessageInfo

func (m *BinaryMessage) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func init() {
	proto.RegisterType((*BinaryMessage)(nil), "BinaryMessage")
}

func init() { proto.RegisterFile("messages.proto", fileDescriptor_4dc296cbfe5ffcd5) }

var fileDescriptor_4dc296cbfe5ffcd5 = []byte{
	// 75 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xcb, 0x4d, 0x2d, 0x2e,
	0x4e, 0x4c, 0x4f, 0x2d, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x57, 0x52, 0xe6, 0xe2, 0x75, 0xca,
	0xcc, 0x4b, 0x2c, 0xaa, 0xf4, 0x85, 0x88, 0x0b, 0x09, 0x71, 0xb1, 0xa4, 0x24, 0x96, 0x24, 0x4a,
	0x30, 0x2a, 0x30, 0x6a, 0xf0, 0x04, 0x81, 0xd9, 0x49, 0x6c, 0x60, 0xb5, 0xc6, 0x80, 0x00, 0x00,
	0x00, 0xff, 0xff, 0xca, 0xc9, 0xa4, 0x6f, 0x3d, 0x00, 0x00, 0x00,
}
