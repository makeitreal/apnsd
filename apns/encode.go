package apns

import (
	"bytes"
	"encoding/gob"
)

func init() {
	gob.Register(Aps{})
	gob.Register(Alert{})
}

func DecodeMsg(byt []byte) (*Msg, error) {
	b := bytes.NewReader(byt)

	dec := gob.NewDecoder(b)
	var msg *Msg
	if err := dec.Decode(&msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func EncodeMsg(msg *Msg) ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	if err := enc.Encode(msg); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
