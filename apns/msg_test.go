package apns

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAutoTrim_AlertBody(t *testing.T) {

	str := "hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string."
	msg := Msg{
		Token:    []byte("hogehogehoge"),
		Priority: 10,
		Expire:   0,
		Payload: Payload{
			"aps": &Aps{
				Alert: &Alert{
					Body: String(str),
				},
				Badge: Int(1),
			},
		},
		Identifier: 1,
	}

	{
		var b bytes.Buffer
		err := msg.write(&b, false)

		if err == nil || err != ErrPayloadLengthOver {
			t.Fatal("should autotorim err. but got:", err)
		}
	}

	{
		var b bytes.Buffer
		err := msg.write(&b, true)

		if err != nil {
			t.Fatal("got err:", err)
		}

		newBody := *msg.Payload["aps"].(*Aps).Alert.(*Alert).Body
		if len(newBody) >= len(str) {
			t.Error("new body should be trimed")
		}

		if !strings.Contains(str, newBody) {
			t.Error("new body should be contain at str")
		}

		if !utf8.ValidString(newBody) {
			t.Error("new body should be valid string")
		}

		t.Log("str:", str)
		t.Log("newBody:", newBody)
	}

}

func TestAutoTrim_AlertString(t *testing.T) {

	str := "hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string.hello 世界. long string."
	msg := Msg{
		Token:    []byte("hogehogehoge"),
		Priority: 10,
		Expire:   0,
		Payload: Payload{
			"aps": &Aps{
				Alert: String(str),
				Badge: Int(1),
			},
		},
		Identifier: 1,
	}

	{
		var b bytes.Buffer
		err := msg.write(&b, false)

		if err == nil || err != ErrPayloadLengthOver {
			t.Fatal("should autotorim err. but got:", err)
		}
	}

	{
		var b bytes.Buffer
		err := msg.write(&b, true)

		if err != nil {
			t.Fatal("got err:", err)
		}

		newBody := *msg.Payload["aps"].(*Aps).Alert.(*string)
		if len(newBody) >= len(str) {
			t.Error("new body should be trimed")
		}

		if !strings.Contains(str, newBody) {
			t.Error("new body should be contain at str")
		}

		if !utf8.ValidString(newBody) {
			t.Error("new body should be valid string")
		}

		t.Log("str:", str)
		t.Log("newBody:", newBody)
	}
}
