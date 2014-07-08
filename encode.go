package apnsd

import (
	"bytes"
	"encoding/gob"

	"github.com/makeitreal/apnsd/apns"
)

func init() {
	gob.Register(&apns.Aps{})
	gob.Register(&apns.Alert{})
}

func DecodeMsg(byt []byte) (*apns.Msg, error) {
	b := bytes.NewReader(byt)

	dec := gob.NewDecoder(b)
	var msg *apns.Msg
	if err := dec.Decode(&msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func EncodeMsg(msg *apns.Msg) ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	if err := enc.Encode(msg); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
