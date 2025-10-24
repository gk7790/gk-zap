package json

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"reflect"
)

func (msgCtl *MsgCtl) unpack(typeByte byte, buffer []byte, msgIn Message) (msg Message, err error) {
	if msgIn == nil {
		t, ok := msgCtl.typeMap[typeByte]
		if !ok {
			err = ErrMsgType
			return
		}

		msg = reflect.New(t).Interface().(Message)
	} else {
		msg = msgIn
	}

	err = json.Unmarshal(buffer, &msg)
	return
}

func (msgCtl *MsgCtl) UnPackInto(buffer []byte, msg Message) (err error) {
	_, err = msgCtl.unpack(' ', buffer, msg)
	return
}

func (msgCtl *MsgCtl) UnPack(typeByte byte, buffer []byte) (msg Message, err error) {
	return msgCtl.unpack(typeByte, buffer, nil)
}

func (msgCtl *MsgCtl) Pack(msg Message) ([]byte, error) {
	typeByte, ok := msgCtl.typeByteMap[reflect.TypeOf(msg).Elem()]
	if !ok {
		return nil, ErrMsgType
	}

	content, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(nil)
	_ = buffer.WriteByte(typeByte)
	_ = binary.Write(buffer, binary.BigEndian, int64(len(content)))
	_, _ = buffer.Write(content)
	return buffer.Bytes(), nil
}
