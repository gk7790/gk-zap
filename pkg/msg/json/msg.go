package json

import (
	"reflect"
)

var defaultMaxMsgLength int64 = 10240

// Message wraps socket packages for communicating between frpc and frps.
type Message interface{}

type MsgCtl struct {
	typeMap     map[byte]reflect.Type
	typeByteMap map[reflect.Type]byte

	maxMsgLength int64
}

func NewMsgCtl() *MsgCtl {
	return &MsgCtl{
		typeMap:      make(map[byte]reflect.Type),
		typeByteMap:  make(map[reflect.Type]byte),
		maxMsgLength: defaultMaxMsgLength,
	}
}

func (msgCtl *MsgCtl) RegisterMsg(typeByte byte, msg interface{}) {
	msgCtl.typeMap[typeByte] = reflect.TypeOf(msg)
	msgCtl.typeByteMap[reflect.TypeOf(msg)] = typeByte
}

func (msgCtl *MsgCtl) SetMaxMsgLength(length int64) {
	msgCtl.maxMsgLength = length
}
