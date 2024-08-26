package p2p

import (
	"bittorent/client"
	"bittorent/message"
	"bittorent/peers"
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"
)

const maxBlockSize = 16384

type Torrent struct {
	Name        string
	Length      int
	PieceLength int
	Peers       chan peers.Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
}

type PieceWork struct {
	index  int
	length int
	hash   [20]byte
}

type PieceProgress struct {
	index      int
	client     *client.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

type PieceResult struct {
	index int
	buf   []byte
}

func (state *PieceProgress) ReadMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	switch msg.ID {
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgHave:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}

	return nil
}

func attemptDownloadPiece(c *client.Client, pw *PieceWork) ([]byte, error) {
	fmt.Println("length", pw.length)
	state := PieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}

	c.Con.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Con.SetDeadline(time.Time{})

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < 5 && state.requested < pw.length {
				blockSize := maxBlockSize

				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}

				err := c.SendRequest(pw.index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}

		err := state.ReadMessage()
		if err != nil {
			return nil, err
		}

	}
	return state.buf, nil
}

func checkIntegrity(pw *PieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if !bytes.Equal(hash[:], pw.hash[:]) {
		return fmt.Errorf("index %d failed integrity check", pw.index)
	}
	return nil
}

func (t *Torrent) startDownloadWorker(peer peers.Peer, workQ chan *PieceWork, results chan *PieceResult) {
	c, err := client.New(t.InfoHash, peer, t.PeerID)
	if err != nil {
		log.Print(err)
		return
	}
	defer c.Con.Close()

	log.Printf("completed handshake with %s", peer.Ip)

	c.SendUnChoke()
	c.SendIntersted()

	for pw := range workQ {
		if !c.Bitfield.HasPiece(pw.index) {
			workQ <- pw
			continue
		}

		buf, err := attemptDownloadPiece(c, pw)
		if err != nil {
			workQ <- pw
			return
		}

		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("Piece #%d failed integrity check\n", pw.index)
			workQ <- pw // Put piece back on the queue
			continue
		}

		c.SendHave(pw.index)
		results <- &PieceResult{pw.index, buf}
	}
}

func (t *Torrent) calculateBoundsForPiece(index int) (begin, end int) {
	begin = index * t.PieceLength
	end = begin + t.PieceLength
	if end > t.Length {
		end = t.Length
	}

	return begin, end
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

func (t *Torrent) Download() ([]byte, error) {
	log.Println("Starting download for", t.Name)

	workQ := make(chan *PieceWork, len(t.PieceHashes))
	results := make(chan *PieceResult)
	doneCh := make(chan struct{})

	for index, hash := range t.PieceHashes {
		length := t.calculatePieceSize(index)
		workQ <- &PieceWork{index, length, hash}
	}

	go func() {
		for {
			select {
			case peer := <-t.Peers:
				go t.startDownloadWorker(peer, workQ, results)
			case <-doneCh:
				close(workQ)
				return
			}
		}
	}()

	buf := make([]byte, t.Length)
	donePieces := 0

	go func() {
		for donePieces < len(t.PieceHashes) {
			res := <-results
			begin, end := t.calculateBoundsForPiece(res.index)

			// save pieces into files instead of memory
			file, _ := os.Create("./downloaded/." + t.Name + strconv.Itoa(res.index) + ".temp")
			defer file.Close()
			file.Write(res.buf)

			copy(buf[begin:end], res.buf)

			donePieces++

			percent := float64(donePieces) / float64(len(t.PieceHashes)) * 100
			numWorkers := runtime.NumGoroutine() - 1 // subtract 1 for main thread
			log.Printf("(%0.2f%%) Downloaded piece #%d from %d peers\n", percent, res.index, numWorkers)
		}
		close(doneCh)
	}()

	<-doneCh

	return buf, nil
}
