package peers

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

type Peer struct {
	Ip   net.IP
	Port uint16
}

func Unmarshal(p []byte) ([]Peer, error) {
	const peerSize = 6
	peerNum := len(p) / peerSize
	if len(p)%peerSize != 0 {
		err := fmt.Errorf("Received malformed peers")
		return nil, err
	}
	peers := make([]Peer, peerNum)

	for i := 0; i < peerNum; i++ {
		offset := i * peerSize
		peers[i].Ip = net.IP(p[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16([]byte(p[offset+4 : offset+6]))
	}
	return peers, nil
}

func (peer Peer) String() string {
	return net.JoinHostPort(peer.Ip.String(), strconv.Itoa(int(peer.Port)))
}
