package apnsd

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"log"
	"net"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/soh335/go-test-redisserver"
	"github.com/ugorji/go/codec"
)

func Test_Apnsd(t *testing.T) {
	mock, err := testNewMockServer()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	redisserver, err := redistest.NewServer(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer redisserver.Stop()

	a := NewApnsd(func() (net.Conn, error) {
		return net.Dial(mock.l.Addr().Network(), mock.l.Addr().String())
	})
	a.RetriverRedisNetwork = "unix"
	a.RetriverRedisAddr = redisserver.Config["unixsocket"]

	aFinished := make(chan struct{}, 1)
	go func() {
		a.Start()
		aFinished <- struct{}{}
	}()

	r, err := redis.Dial(a.RetriverRedisNetwork, a.RetriverRedisAddr)
	if err != nil {
		t.Fatal(err)
	}

	////////////
	// normal //
	////////////
	{
		payload := NewPayload(&Aps{
			Alert: String("hoge"),
			Badge: Int(10),
		})
		payload["extra"] = "hoge"
		token := make([]byte, DeviceTokenLength)
		rand.Read(token)
		expire := uint32(time.Now().Unix())
		msg, err := NewMsg(token, payload, expire, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := codec.NewEncoder(&b, &codec.MsgpackHandle{}).Encode(msg); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Do("LPUSH", a.RetriveKey, b.Bytes()); err != nil {
			t.Fatal(err)
		}

		mockMsg := <-mock.msgChan

		if mockMsg.Expire != expire {
			t.Errorf("expire should %d but %d", expire, mockMsg.Expire)
		}

		if expected := uint32(1); mockMsg.Identifier != expected {
			t.Errorf("identifier should %d but %d", expected, mockMsg.Identifier)
		}

		if string(mockMsg.Token) != string(msg.Token) {
			t.Errorf("token should %s but %s", msg.Token, mockMsg.Token)
		}

		if expected := `{"aps":{"alert":"hoge","badge":10},"extra":"hoge"}`; mockMsg.Payload != expected {
			t.Errorf("payload should be %s but %s", expected, mockMsg.Payload)
		}
	}

	////////////////////////////
	// alert.body and 0 badge //
	////////////////////////////
	{
		payload := NewPayload(&Aps{
			Alert: &Alert{
				Body: String("hoge"),
			},
			Badge: Int(0),
		})
		token := make([]byte, DeviceTokenLength)
		rand.Read(token)
		expire := uint32(time.Now().Unix())
		msg, err := NewMsg(token, payload, expire, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := codec.NewEncoder(&b, &codec.MsgpackHandle{}).Encode(msg); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Do("LPUSH", a.RetriveKey, b.Bytes()); err != nil {
			t.Fatal(err)
		}

		mockMsg := <-mock.msgChan

		if expected := uint32(2); mockMsg.Identifier != expected {
			t.Errorf("identifier should %d but %d", expected, mockMsg.Identifier)
		}

		if expected := `{"aps":{"alert":{"body":"hoge"},"badge":0}}`; mockMsg.Payload != expected {
			t.Errorf("payload should be %s but %s", expected, mockMsg.Payload)
		}
	}

	/////////////////////
	// long alert.body //
	/////////////////////
	{
		payload := NewPayload(&Aps{
			Alert: &Alert{
				Body: String("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"),
			},
			Badge: Int(0),
		})
		token := make([]byte, DeviceTokenLength)
		rand.Read(token)
		expire := uint32(time.Now().Unix())
		msg, err := NewMsg(token, payload, expire, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := codec.NewEncoder(&b, &codec.MsgpackHandle{}).Encode(msg); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Do("LPUSH", a.RetriveKey, b.Bytes()); err != nil {
			t.Fatal(err)
		}

		mockMsg := <-mock.msgChan

		if expected := uint32(3); mockMsg.Identifier != expected {
			t.Errorf("identifier should %d but %d", expected, mockMsg.Identifier)
		}

		if got := len(mockMsg.Payload); got > MaxPayloadLength {
			t.Errorf("payload length %d should be little than %d", got, MaxPayloadLength)
		}
	}

	////////////////////////////////////////////////////
	// disconnect from apns server and auto reconnect //
	////////////////////////////////////////////////////
	{
		mock.shutdownChan <- struct{}{}

		time.Sleep(time.Second * 3)

		if expected := int32(2); a.numOfConnectToApns != expected {
			t.Errorf("numOfConnectToApns should be %d but %d", expected, a.numOfConnectToApns)
		}
	}

	//////////////
	// shutdown //
	//////////////
	{
		a.shutdownChan <- struct{}{}

		select {
		case <-aFinished:
		case <-time.After(time.Second * 5):
			t.Errorf("long shutdown...")
		}
	}
}

type testMockApnsServer struct {
	l            net.Listener
	shutdownChan chan struct{}
	msgChan      chan *Msg
}

func testNewMockServer() (*testMockApnsServer, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	t := &testMockApnsServer{
		l:            l,
		msgChan:      make(chan *Msg, 10),
		shutdownChan: make(chan struct{}),
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Println("mock server accept err:", err)
				return
			}

			go func(conn net.Conn) {
				defer conn.Close()
				go func() {
					<-t.shutdownChan
					log.Println("mock server force connection close")
					conn.Close()
				}()

				for {
					var command uint8
					var frameLength uint32

					if err := binary.Read(conn, binary.BigEndian, &command); err != nil {
						log.Println(err)
						return
					}
					if err := binary.Read(conn, binary.BigEndian, &frameLength); err != nil {
						log.Println(err)
						return
					}

					frame := make([]byte, frameLength)

					if err := binary.Read(conn, binary.BigEndian, &frame); err != nil {
						log.Println(err)
						return
					}

					msg, err := t.DecodeMsg(frame)
					if err != nil {
						log.Println(err)
						return
					}

					t.msgChan <- msg
				}
			}(conn)
		}
	}()

	return t, nil
}

func (t *testMockApnsServer) Close() {
	close(t.shutdownChan)
	t.l.Close()
}

func (t *testMockApnsServer) DecodeMsg(frame []byte) (*Msg, error) {
	f := bytes.NewReader(frame)

	// device token
	var deviceTokenId uint8
	var deviceTokenLength uint16

	binary.Read(f, binary.BigEndian, &deviceTokenId)
	binary.Read(f, binary.BigEndian, &deviceTokenLength)

	deviceToken := make([]byte, deviceTokenLength)

	binary.Read(f, binary.BigEndian, &deviceToken)

	// payload
	var payloadItemId uint8
	var payloadLength uint16

	binary.Read(f, binary.BigEndian, &payloadItemId)
	binary.Read(f, binary.BigEndian, &payloadLength)

	payload := make([]byte, payloadLength)

	binary.Read(f, binary.BigEndian, &payload)
	log.Println(string(payload))

	// notification identifier
	var notificationIdentifierItemId uint8
	var notificationIdentifierLength uint16
	var notificationIdentifier uint32

	binary.Read(f, binary.BigEndian, &notificationIdentifierItemId)
	binary.Read(f, binary.BigEndian, &notificationIdentifierLength)
	binary.Read(f, binary.BigEndian, &notificationIdentifier)

	// expiration date
	var expirationDateItemId uint8
	var expirationDateLength uint16
	var expiration uint32

	binary.Read(f, binary.BigEndian, &expirationDateItemId)
	binary.Read(f, binary.BigEndian, &expirationDateLength)
	binary.Read(f, binary.BigEndian, &expiration)

	// priority
	var priorityItemId uint8
	var priorityLength uint16
	var priority uint8

	binary.Read(f, binary.BigEndian, &priorityItemId)
	binary.Read(f, binary.BigEndian, &priorityLength)
	binary.Read(f, binary.BigEndian, &priority)

	return &Msg{
		Token:      deviceToken,
		Payload:    string(payload),
		Expire:     expiration,
		Priority:   priority,
		Identifier: notificationIdentifier,
	}, nil
}
