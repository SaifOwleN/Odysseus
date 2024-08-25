package torrent

import (
	"bittorent/peers"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
)

type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func BuildTrackerUrl(t *TorrentFile, peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}
	base.RawQuery = params.Encode()

	return base.String(), nil
}

func (t *TorrentFile) RequestPeers(peerID [20]byte, port uint16, peersChan chan peers.Peer) error {
	url, err := BuildTrackerUrl(t, peerID, port)
	if err != nil {
		return err
	}

	c := &http.Client{Timeout: 15 * time.Second}

	log.Print(url)

	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	trackerResp := bencodeTrackerResp{}

	err = bencode.Unmarshal(resp.Body, &trackerResp)
	if err != nil {
		return err
	}

	fmt.Println("help ", trackerResp)

	peers, err := peers.Unmarshal([]byte(trackerResp.Peers))

	for _, peer := range peers {
		peersChan <- peer
	}

	return nil
}
