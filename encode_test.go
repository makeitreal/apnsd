package apnsd

import (
	"reflect"
	"testing"

	"github.com/makeitreal/apnsd/apns"
)

func TestEncodeDecode(t *testing.T) {

	orgMsg := &apns.Msg{
		Token:    []byte("hoge"),
		Priority: 10,
		Expire:   0,
		Payload: apns.Payload{
			"aps": &apns.Aps{
				Alert: &apns.Alert{
					Body: apns.String("hello!"),
				},
				Badge: apns.Int(1),
			},
		},
	}

	byt, err := EncodeMsg(orgMsg)

	if err != nil {
		t.Fatal(err)
	}

	newMsg, err := DecodeMsg(byt)

	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(orgMsg, newMsg) {
		t.Error("fail deep equal orgMsg:", orgMsg, " newMsg:", newMsg)
	}
}
