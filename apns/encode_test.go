package apns

import (
	"encoding/json"
	"testing"
)

func TestEncodeDecode(t *testing.T) {

	orgMsg := &Msg{
		Token:    []byte("hoge"),
		Priority: 10,
		Expire:   12345,
		Payload: Payload{
			"aps": &Aps{
				Alert: &Alert{
					Body: String("hello!"),
				},
				Badge: Int(0),
				Sound: String(""),
			},
			"fuga":     "foo",
			"dameleon": 1,
			"empty":    "",
			"pointer":  String(""),
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

	if string(orgMsg.Token) != string(newMsg.Token) {
		t.Error("orgMsg.Token", orgMsg.Token, "newMsg.Token", newMsg.Token)
	}

	if orgMsg.Priority != newMsg.Priority {
		t.Error("orgMsg.Priority", orgMsg.Priority, "newMsg.Priority", newMsg.Priority)
	}

	if orgMsg.Expire != newMsg.Expire {
		t.Error("orgMsg.Expire", orgMsg.Expire, "newMsg.Expire", newMsg.Expire)
	}

	// payload

	{
		byt, err := json.Marshal(newMsg.Payload)
		if err != nil {
			t.Fatal(err)
		}

		if string(byt) != `{"aps":{"alert":{"body":"hello!"},"badge":0,"sound":""},"dameleon":1,"empty":"","fuga":"foo","pointer":""}` {
			t.Error("null value should not cutoff")
			t.Log(string(byt))
		}
	}
}
