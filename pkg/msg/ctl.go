package msg

import (
	"io"

	jMsg "github.com/gk7790/gk-zap/pkg/msg/json"
)

type Message = jMsg.Message

var msgCtl *jMsg.MsgCtl

func init() {
	msgCtl = jMsg.NewMsgCtl()
	for typeByte, msg := range msgTypeMap {
		msgCtl.RegisterMsg(typeByte, msg)
	}
}

func ReadMsg(c io.Reader) (msg Message, err error) {
	return msgCtl.ReadMsg(c)
}

func ReadMsgInto(c io.Reader, msg Message) (err error) {
	return msgCtl.ReadMsgInto(c, msg)
}

func WriteMsg(c io.Writer, msg any) (err error) {
	return msgCtl.WriteMsg(c, msg)
}
