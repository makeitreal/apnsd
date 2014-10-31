package apnsd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	DeviceTokenItemId            = 1
	DeviceTokenLength            = 32
	PayloadItemId                = 2
	NotificationIdentifierItemId = 3
	NotificationIdentifierLength = 4
	ExpirationDateItemId         = 4
	ExpirationDateLength         = 4
	PriorityItemId               = 5
	PriorityLength               = 1

	MaxPayloadLength = 256
)

type Payload map[string]interface{}

func NewPayload(aps *Aps) Payload {
	p := Payload{}
	p["aps"] = aps
	return p
}

type Msg struct {
	Token      []byte `codec:"token"`
	Payload    string `codec:"payload"`
	Expire     uint32 `codec:"expire, omitempty"`
	Priority   uint8  `codec:"priority, omitempty"`
	Identifier uint32 `codec:"identifier, omitempty"`
}

func NewMsg(token []byte, payload Payload, expire uint32, priority uint8, identifier uint32) (*Msg, error) {
	byt, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	m := Msg{}
	m.Token = token
	m.Payload = string(byt)
	m.Expire = expire
	m.Priority = priority
	m.Identifier = identifier

	return &m, nil
}

func (m *Msg) WriteWithAutotrim(w io.Writer) error {
	trimedPayload, err := m.payloadBytWithAutotrim()
	if err != nil {
		return err
	}

	var b bytes.Buffer

	// device token
	binary.Write(&b, binary.BigEndian, uint8(DeviceTokenItemId))
	binary.Write(&b, binary.BigEndian, uint16(DeviceTokenLength))
	binary.Write(&b, binary.BigEndian, m.Token)

	// payload
	binary.Write(&b, binary.BigEndian, uint8(PayloadItemId))
	binary.Write(&b, binary.BigEndian, uint16(len(trimedPayload)))
	binary.Write(&b, binary.BigEndian, trimedPayload)

	// nofication identifier
	binary.Write(&b, binary.BigEndian, uint8(NotificationIdentifierItemId))
	binary.Write(&b, binary.BigEndian, uint16(NotificationIdentifierLength))
	binary.Write(&b, binary.BigEndian, m.Identifier)

	// expiration date
	binary.Write(&b, binary.BigEndian, uint8(ExpirationDateItemId))
	binary.Write(&b, binary.BigEndian, uint16(ExpirationDateLength))
	binary.Write(&b, binary.BigEndian, m.Expire)

	// priority
	binary.Write(&b, binary.BigEndian, uint8(PriorityItemId))
	binary.Write(&b, binary.BigEndian, uint16(PriorityLength))
	binary.Write(&b, binary.BigEndian, m.Priority)

	// frame
	binary.Write(w, binary.BigEndian, uint8(2))        // command
	binary.Write(w, binary.BigEndian, uint32(b.Len())) // frame length
	binary.Write(w, binary.BigEndian, b.Bytes())       // frame

	return nil
}

func (m *Msg) payloadBytWithAutotrim() ([]byte, error) {
	over := len(m.Payload) - MaxPayloadLength

	if over > 0 {
		mp := map[string]interface{}{}
		if err := json.NewDecoder(strings.NewReader(m.Payload)).Decode(&mp); err != nil {
			return nil, err
		}

		switch mp["aps"].(map[string]interface{})["alert"].(type) {
		case string:
			trim, err := m.trim(mp["aps"].(map[string]interface{})["alert"].(string), over)
			if err != nil {
				return nil, err
			}
			mp["aps"].(map[string]interface{})["alert"] = trim
		default:
			switch mp["aps"].(map[string]interface{})["alert"].(map[string]interface{})["body"].(type) {
			case string:
				trim, err := m.trim(mp["aps"].(map[string]interface{})["alert"].(map[string]interface{})["body"].(string), over)
				if err != nil {
					return nil, err
				}
				mp["aps"].(map[string]interface{})["alert"].(map[string]interface{})["body"] = trim
			default:
				return nil, fmt.Errorf("aps.alert or aps.alert.body should be string")
			}
		}

		byt, err := json.Marshal(mp)
		if err != nil {
			return nil, err
		}

		return byt, nil
	}

	return []byte(m.Payload), nil
}

func (m *Msg) trim(str string, over int) (string, error) {
	byt := []byte(str)

	for i := len(byt) - 1 - over; i >= 0; i-- {
		if subByt := byt[0:i]; utf8.Valid(subByt) {
			return string(subByt), nil
		}
	}

	return "", fmt.Errorf("trim error: ordinal str:%s", str)
}

type Aps struct {
	Alert            interface{} `json:"alert,omitempty"`
	Badge            *int        `json:"badge,omitempty"`
	Sound            *string     `json:"sound,omitempty"`
	ContentAvailable *string     `json:"content-available,omitempty"`
}

type Alert struct {
	Body         *string  `json:"body,omitempty"`
	ActionLocKey *string  `json:"action-loc-key,omitempty"`
	LocKey       *string  `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  *string  `json:"launch-image,omitempty"`
}

type ErrorMsg struct {
	Command    uint8
	Status     uint8
	Identifier uint32
}

func (e *ErrorMsg) Read(r io.Reader) error {
	b := make([]byte, 6)
	_, err := r.Read(b)
	if err != nil && err != io.EOF {
		return err
	}

	br := bytes.NewReader(b)

	binary.Read(br, binary.BigEndian, &e.Command)
	binary.Read(br, binary.BigEndian, &e.Status)
	binary.Read(br, binary.BigEndian, &e.Identifier)
	return nil
}

func String(v string) *string {
	s := new(string)
	*s = v
	return s
}

func Int(v int) *int {
	i := new(int)
	*i = v
	return i
}

type FailedMsg struct {
	*Msg
	Status uint8 `codec:"status"`
}
