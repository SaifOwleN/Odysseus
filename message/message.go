package message

import (
	"encoding/binary"
	"fmt"
	"io"
)

type messageID uint8

const (
	// MsgChoke chokes the receiver
	MsgChoke messageID = 0
	// MsgUnchoke unchokes the receiver
	MsgUnchoke messageID = 1
	// MsgInterested expresses interest in receiving data
	MsgInterested messageID = 2
	// MsgNotInterested expresses disinterest in receiving data
	MsgNotInterested messageID = 3
	// MsgHave alerts the receiver that the sender has downloaded a piece
	MsgHave messageID = 4
	// MsgBitfield encodes which pieces that the sender has downloaded
	MsgBitfield messageID = 5
	// MsgRequest requests a block of data from the receiver
	MsgRequest messageID = 6
	// MsgPiece delivers a block of data to fulfill a request
	MsgPiece messageID = 7
	// MsgCancel cancels a request
	MsgCancel messageID = 8
)

type Message struct {
	ID      messageID
	Payload []byte
}

func CreateRequest(index, begin, length int) *Message {
	buf := make([]byte, 12)

	binary.BigEndian.PutUint32(buf[0:4], uint32(index))
	binary.BigEndian.PutUint32(buf[4:8], uint32(begin))
	binary.BigEndian.PutUint32(buf[8:12], uint32(length))

	return &Message{
		6,
		buf,
	}
}

func ParsePiece(index int, buf []byte, msg *Message) (int, error) {
	ind := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
	if ind != index {
		return 0, fmt.Errorf("index mismatch")
	}

	begin := int(binary.BigEndian.Uint32(msg.Payload[4:8]))

	if begin >= len(buf) {
		return 0, fmt.Errorf("begin offset higher than buffer size")
	}
	data := msg.Payload[8:]

	if begin+len(data) > len(buf) {
		return 0, fmt.Errorf("data too long [%d] for offset %d with length %d", len(data), begin, len(buf))
	}

	copy(buf[begin:], data)
	return len(data), nil
}

func CreateHave(index int) *Message {
	buf := make([]byte, 4)

	binary.BigEndian.PutUint32(buf[0:4], uint32(index))

	return &Message{
		4,
		buf,
	}
}

func ParseHave(msg *Message) (int, error) {
	if msg.ID != MsgHave {
		return 0, nil
	}
	if len(msg.Payload) != 4 {
		return 0, nil
	}
	return int(binary.BigEndian.Uint32(msg.Payload[0:4])), nil
}

// Serialize serializes a message into a buffer of the form
// <length prefix><message ID><payload>
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}
	length := uint32(len(m.Payload) + 1)
	buf := make([]byte, 4+length) // +4 bytes for the 32 bit of the length
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

// Read takes a message from the stream and parses it

func Read(r io.Reader) (*Message, error) {
	lenBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lenBuf)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)

	if length == 0 {
		return nil, nil
	}

	buf := make([]byte, length)

	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	messsage := Message{
		ID:      messageID(buf[0]),
		Payload: buf[1:],
	}
	return &messsage, nil
}
