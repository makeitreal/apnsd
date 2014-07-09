package apns

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
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

type Msg struct {
	Token      []byte
	Payload    Payload
	Expire     uint32
	Priority   uint8
	Identifier uint32
}

func (m *Msg) write(w io.Writer) error {

	payload, err := json.Marshal(m.Payload)
	if err != nil {
		return err
	}

	//TODO: auto trim
	if len(payload) > MaxPayloadLength {
		return errors.New("payload length is over")
	}

	var b bytes.Buffer

	// device token
	binary.Write(&b, binary.BigEndian, uint8(DeviceTokenItemId))
	binary.Write(&b, binary.BigEndian, uint16(DeviceTokenLength))
	binary.Write(&b, binary.BigEndian, m.Token)

	// payload
	binary.Write(&b, binary.BigEndian, uint8(PayloadItemId))
	binary.Write(&b, binary.BigEndian, uint16(len(payload)))
	binary.Write(&b, binary.BigEndian, payload)

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

type Aps struct {
	Alert            interface{} `json:"alert,omitempty"`
	Badge            *int        `json:"badge,omitempty"`
	Sound            *string     `json:"sound,omitempty"`
	ContentAvailable *string     `json:"content-available,omitempty"`
}

//TODO: more good idea
func (a *Aps) GobDecode(byt []byte) error {
	return json.Unmarshal(byt, a)
}

func (a *Aps) GobEncode() ([]byte, error) {
	return json.Marshal(a)
}

type Alert struct {
	Body         *string  `json:"body,omitempty"`
	ActionLocKey *string  `json:"action-loc-key,omitempty"`
	LocKey       *string  `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  *string  `json:"launch-image,omitempty"`
}

func (a *Alert) GobDecode(byt []byte) error {
	return json.Unmarshal(byt, a)
}

func (a *Alert) GobEncode() ([]byte, error) {
	return json.Marshal(a)
}

type ErrorMsg struct {
	Command    uint8
	Status     uint8
	Identifier uint32
}

func (e *ErrorMsg) Read(r io.Reader) {
	binary.Read(r, binary.BigEndian, &e.Command)
	binary.Read(r, binary.BigEndian, &e.Status)
	binary.Read(r, binary.BigEndian, &e.Identifier)
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
