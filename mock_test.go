package apnsd

import (
	"bufio"
	"bytes"
	"crypto/rsa"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"
	"net"
	"testing"

	"github.com/makeitreal/apnsd/apns"
)

type testMockApnsServer struct {
	l         net.Listener
	tlsConfig *tls.Config
	msgChan   chan *apns.Msg
}

func newTestMockApnsServer() (*testMockApnsServer, error) {
	config := &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	config.Certificates[0].Certificate = [][]byte{testRSACertificate}
	config.Certificates[0].PrivateKey = testRSAPrivateKey

	l, err := tls.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"), config)

	if err != nil {
		return nil, err
	}

	m := &testMockApnsServer{}
	m.tlsConfig = config
	m.l = l
	m.msgChan = make(chan *apns.Msg, 10)

	return m, nil
}

//TODO: error handliong
func (m *testMockApnsServer) handle() error {

	for {
		conn, err := m.l.Accept()

		if err != nil {
			return err
		}

		go func(conn net.Conn) {
			for {
				r := bufio.NewReader(conn)
				var command uint8
				var frameLength uint32

				if err := binary.Read(r, binary.BigEndian, &command); err != nil {
					log.Println(err)
					return
				}
				if err := binary.Read(r, binary.BigEndian, &frameLength); err != nil {
					log.Println(err)
					return
				}

				frame := make([]byte, frameLength)

				if err := binary.Read(r, binary.BigEndian, &frame); err != nil {
					log.Println(err)
					return
				}

				msg, err := testApnsDecodeBinary(frame)
				if err != nil {
					log.Println(err)
					return
				}

				m.msgChan <- msg
			}
		}(conn)
	}

	return nil
}

func testApnsDecodeBinary(frame []byte) (*apns.Msg, error) {
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

	var p apns.Payload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}

	return &apns.Msg{
		Token:      deviceToken,
		Payload:    p,
		Expire:     expiration,
		Priority:   priority,
		Identifier: notificationIdentifier,
	}, nil
}

func TestMockServer(t *testing.T) {

	mock, err := newTestMockApnsServer()
	if err != nil {
		t.Fatal(err)
	}

	defer mock.l.Close()

	go mock.handle()

	tlsConn, err := tls.Dial("tcp", mock.l.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	conn := apns.NewConnection(tlsConn)

	hexToken, err := hex.DecodeString("1234567890123456789123456789123456789123456789123456789123456789")
	if err != nil {
		t.Fatal(err)
	}

	orgMsg := &apns.Msg{
		Token:    hexToken,
		Priority: 13,
		Expire:   0,
		Payload: apns.Payload{
			"aps": &apns.Aps{
				Alert: &apns.Alert{
					Body: apns.String("hi"),
				},
				Badge: apns.Int(0),
			},
			"dame": "leon",
		},
	}

	if err := conn.WriteMsg(orgMsg); err != nil {
		t.Fatal(err)
	}

	newMsg := <-mock.msgChan

	if newMsg.Payload["dame"] != "leon" {
		t.Log("orgMsg:", orgMsg)
		t.Log("newMsg:", newMsg)
		t.Error("failed decode frame")
	}
}

// TODO: separate license ?
// taken from crypto/tls/handshake_server_test.go

// Copyright (c) 2012 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

func bigFromString(s string) *big.Int {
	ret := new(big.Int)
	ret.SetString(s, 10)
	return ret
}

func fromHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

var testRSACertificate = fromHex("308202b030820219a00302010202090085b0bba48a7fb8ca300d06092a864886f70d01010505003045310b3009060355040613024155311330110603550408130a536f6d652d53746174653121301f060355040a1318496e7465726e6574205769646769747320507479204c7464301e170d3130303432343039303933385a170d3131303432343039303933385a3045310b3009060355040613024155311330110603550408130a536f6d652d53746174653121301f060355040a1318496e7465726e6574205769646769747320507479204c746430819f300d06092a864886f70d010101050003818d0030818902818100bb79d6f517b5e5bf4610d0dc69bee62b07435ad0032d8a7a4385b71452e7a5654c2c78b8238cb5b482e5de1f953b7e62a52ca533d6fe125c7a56fcf506bffa587b263fb5cd04d3d0c921964ac7f4549f5abfef427100fe1899077f7e887d7df10439c4a22edb51c97ce3c04c3b326601cfafb11db8719a1ddbdb896baeda2d790203010001a381a73081a4301d0603551d0e04160414b1ade2855acfcb28db69ce2369ded3268e18883930750603551d23046e306c8014b1ade2855acfcb28db69ce2369ded3268e188839a149a4473045310b3009060355040613024155311330110603550408130a536f6d652d53746174653121301f060355040a1318496e7465726e6574205769646769747320507479204c746482090085b0bba48a7fb8ca300c0603551d13040530030101ff300d06092a864886f70d010105050003818100086c4524c76bb159ab0c52ccf2b014d7879d7a6475b55a9566e4c52b8eae12661feb4f38b36e60d392fdf74108b52513b1187a24fb301dbaed98b917ece7d73159db95d31d78ea50565cd5825a2d5a5f33c4b6d8c97590968c0f5298b5cd981f89205ff2a01ca31b9694dda9fd57e970e8266d71999b266e3850296c90a7bdd9")

var testRSAPrivateKey = &rsa.PrivateKey{
	PublicKey: rsa.PublicKey{
		N: bigFromString("131650079503776001033793877885499001334664249354723305978524647182322416328664556247316495448366990052837680518067798333412266673813370895702118944398081598789828837447552603077848001020611640547221687072142537202428102790818451901395596882588063427854225330436740647715202971973145151161964464812406232198521"),
		E: 65537,
	},
	D: bigFromString("29354450337804273969007277378287027274721892607543397931919078829901848876371746653677097639302788129485893852488285045793268732234230875671682624082413996177431586734171663258657462237320300610850244186316880055243099640544518318093544057213190320837094958164973959123058337475052510833916491060913053867729"),
	Primes: []*big.Int{
		bigFromString("11969277782311800166562047708379380720136961987713178380670422671426759650127150688426177829077494755200794297055316163155755835813760102405344560929062149"),
		bigFromString("10998999429884441391899182616418192492905073053684657075974935218461686523870125521822756579792315215543092255516093840728890783887287417039645833477273829"),
	},
}
