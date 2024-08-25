package client

import (
	"bittorent/bitfield"
	"bittorent/handshake"
	"bittorent/message"
	"bittorent/peers"
	"bytes"
	"fmt"
	"net"
	"time"
)

type Client struct {
	Peer     peers.Peer
	PeerID   [20]byte
	InfoHash [20]byte
	Bitfield bitfield.Bitfield
	Choked   bool
	Con      net.Conn
}

func CompleteHandshake(conn net.Conn, InfoHash [20]byte, peerID [20]byte) (*handshake.Handshake, error) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})

	newHandshake := handshake.New(InfoHash, peerID)

	serialized := newHandshake.Serialize()
	_, err := conn.Write(serialized)
	if err != nil {
		return nil, err
	}

	res, err := handshake.Read(conn)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(res.InfoHash[:], InfoHash[:]) {
		return nil, fmt.Errorf("different info hash recieved (got %x, expected %x) ", res.InfoHash, InfoHash)
	}

	return res, nil
}

func recvBitfield(conn net.Conn) (bitfield.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})

	msg, err := message.Read(conn)
	if err != nil {
		return nil, err
	}

	if msg.ID != message.MsgBitfield {
		return nil, fmt.Errorf("didnt recieve a bitfield but got msg of id %d", msg.ID)
	}

	return msg.Payload, nil
}

func New(InfoHash [20]byte, peer peers.Peer, peerID [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 10*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = CompleteHandshake(conn, InfoHash, peerID)
	if err != nil {
		conn.Close()
		return nil, err
	}

	bf, err := recvBitfield(conn)
	if err != nil {
		return nil, err
	}

	client := Client{
		peer,
		peerID,
		InfoHash,
		bf,
		true,
		conn,
	}
	return &client, nil
}

func (c *Client) Read() (*message.Message, error) {
	msg, err := message.Read(c.Con)

	return msg, err
}

func (c *Client) SendRequest(index, begin, length int) error {
	msg := message.CreateRequest(index, begin, length)
	_, err := c.Con.Write(msg.Serialize())
	return err
}

func (c *Client) SendHave(index int) error {
	msg := message.CreateHave(index)
	_, err := c.Con.Write(msg.Serialize())
	return err
}

func (c *Client) SendIntersted() error {
	msg := message.Message{ID: message.MsgInterested}
	_, err := c.Con.Write(msg.Serialize())
	return err
}

func (c *Client) SendNotIntersted() error {
	msg := message.Message{ID: message.MsgNotInterested}
	_, err := c.Con.Write(msg.Serialize())
	return err
}

func (c *Client) SendUnChoke() error {
	msg := message.Message{ID: message.MsgUnchoke}
	_, err := c.Con.Write(msg.Serialize())
	return err
}
