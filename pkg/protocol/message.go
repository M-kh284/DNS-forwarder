package protocol

import (
	"encoding/binary"
	"errors"
	"time"
)

// MessageType نوع پیام‌های تبادلی
type MessageType byte

const (
	// TypeDNSQuery درخواست DNS
	TypeDNSQuery MessageType = 0x01
	// TypeDNSResponse پاسخ DNS
	TypeDNSResponse MessageType = 0x02
	// TypeHeartbeat پیام زنده بودن
	TypeHeartbeat MessageType = 0x03
	// TypeHeartbeatAck تایید زنده بودن
	TypeHeartbeatAck MessageType = 0x04
)

// Message ساختار پیام تانل
type Message struct {
	Type      MessageType
	RequestID uint32
	Timestamp int64
	Payload   []byte
}

// Encode تبدیل پیام به بایت
func (m *Message) Encode() []byte {
	// Format: Type(1) + RequestID(4) + Timestamp(8) + PayloadLen(4) + Payload
	buf := make([]byte, 17+len(m.Payload))

	buf[0] = byte(m.Type)
	binary.BigEndian.PutUint32(buf[1:5], m.RequestID)
	binary.BigEndian.PutUint64(buf[5:13], uint64(m.Timestamp))
	binary.BigEndian.PutUint32(buf[13:17], uint32(len(m.Payload)))
	copy(buf[17:], m.Payload)

	return buf
}

// Decode تبدیل بایت به پیام
func Decode(data []byte) (*Message, error) {
	if len(data) < 17 {
		return nil, errors.New("message too short")
	}

	msg := &Message{
		Type:      MessageType(data[0]),
		RequestID: binary.BigEndian.Uint32(data[1:5]),
		Timestamp: int64(binary.BigEndian.Uint64(data[5:13])),
	}

	payloadLen := binary.BigEndian.Uint32(data[13:17])
	if len(data) < 17+int(payloadLen) {
		return nil, errors.New("payload incomplete")
	}

	msg.Payload = make([]byte, payloadLen)
	copy(msg.Payload, data[17:17+payloadLen])

	return msg, nil
}

// NewDNSQuery ایجاد پیام درخواست DNS
func NewDNSQuery(requestID uint32, dnsPacket []byte) *Message {
	return &Message{
		Type:      TypeDNSQuery,
		RequestID: requestID,
		Timestamp: time.Now().UnixNano(),
		Payload:   dnsPacket,
	}
}

// NewDNSResponse ایجاد پیام پاسخ DNS
func NewDNSResponse(requestID uint32, dnsPacket []byte) *Message {
	return &Message{
		Type:      TypeDNSResponse,
		RequestID: requestID,
		Timestamp: time.Now().UnixNano(),
		Payload:   dnsPacket,
	}
}

// NewHeartbeat ایجاد پیام heartbeat
func NewHeartbeat() *Message {
	return &Message{
		Type:      TypeHeartbeat,
		RequestID: 0,
		Timestamp: time.Now().UnixNano(),
	}
}

// NewHeartbeatAck ایجاد پیام تایید heartbeat
func NewHeartbeatAck() *Message {
	return &Message{
		Type:      TypeHeartbeatAck,
		RequestID: 0,
		Timestamp: time.Now().UnixNano(),
	}
}
