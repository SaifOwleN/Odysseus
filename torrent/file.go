package torrent

import (
	"bittorent/p2p"
	"bittorent/peers"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
)

const Port uint16 = 6881

type BencodeInfo struct {
	Length      int        `bencode:"length,omitempty"`
	Files       []FileInfo `bencode:"files,omitempty"`
	Name        string     `bencode:"name"`
	PieceLength int        `bencode:"piece length"`
	Pieces      string     `bencode:"pieces"`
}

type FileInfo struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

type BencodeTorrent struct {
	Info         BencodeInfo `bencode:"info"`
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list,omitempty"`
}

type TorrentFile struct {
	Announce     string
	AnnounceList [][]string
	InfoHash     [20]byte
	PiecesHash   [][20]byte
	Length       int
	Files        []FileInfo
	Name         string
	PieceLength  int
	PiecesNum    int
}

func (t *TorrentFile) DownloadToFile(path string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}

	peersChan := make(chan peers.Peer)

	go func() {
		for i := 1; ; i++ {
			go func() {
				err := t.RequestPeers(peerID, Port, peersChan)
				if err != nil {
					log.Printf("Error requesting peers: %v", err)
				}
			}()
			time.Sleep(2 * time.Second)
		}
		// close(peersChan) // Close the channel once all peer requests are done
	}()

	length := t.Length

	torrent := p2p.Torrent{
		Peers:       peersChan,
		PeerID:      peerID,
		InfoHash:    t.InfoHash,
		PieceHashes: t.PiecesHash,
		PieceLength: t.PieceLength,
		Length:      length,
		Name:        t.Name,
	}
	buf, err := torrent.Download()
	if err != nil {
		return err
	}

	if len(t.Files) > 0 {
		err := os.MkdirAll(t.Name, 0755)
		if err != nil {
			return err
		}

		begin := 0
		end := 0

		for _, file := range t.Files {

			outFile, err := os.Create(t.Name + "/" + file.Path[0])
			if err != nil {
				return err
			}

			end = begin + file.Length
			if end > len(buf) {
				end = len(buf)
			}

			pieceIndex := int(math.Floor(float64(begin) / float64(t.PieceLength)))
			pieceStart := begin - (pieceIndex * t.PieceLength) // start byte of the first piece
			spanLen := int(math.Ceil(float64(file.Length) / float64(t.PieceLength)))
			pieceEnd := end - (t.PieceLength * (pieceIndex + spanLen - 1)) // end byte of the last piece
			fmt.Println("pieceIndex", pieceIndex)
			fmt.Println("pieceStart", pieceStart)
			fmt.Println("spanLen", spanLen)
			fmt.Println("pieceEnd", pieceEnd)

			for in := range spanLen {
				piece, _ := os.ReadFile("./downloaded/." + t.Name + strconv.Itoa(in+pieceIndex) + ".temp")
				start := 0
				last := len(piece)
				if in == 0 {
					start = int(math.Max(float64(pieceStart), 0))
				}

				if in == spanLen-1 {
					last = pieceEnd
				}
				// fmt.Printf("begin %d, end %d \n", start, last)

				_, err := outFile.Write(piece[start:last])
				if err != nil {
					outFile.Close()
					return err
				}
				// fmt.Print(strconv.Itoa(in) + " | ")
			}

			// _, err = outFile.Write(buf[begin:end])
			// if err != nil {
			// 	outFile.Close()
			// 	return err
			// }
			begin = end
			outFile.Close()
		}
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}

	_, err = outFile.Write(buf)
	if err != nil {
		return err
	}
	return nil
}

func Open(r string) (TorrentFile, error) {
	file, err := os.Open(r)
	if err != nil {
		return TorrentFile{}, err
	}
	defer file.Close()

	bto := BencodeTorrent{}
	err = bencode.Unmarshal(file, &bto)
	if err != nil {
		return TorrentFile{}, err
	}
	return bto.ToTorrentFile()
}

func (t *BencodeInfo) Hash() ([20]byte, error) {
	var buf bytes.Buffer

	err := bencode.Marshal(&buf, *t)
	if err != nil {
		return [20]byte{}, err
	}

	h := sha1.Sum(buf.Bytes())
	fmt.Printf("sha1 infoHash %x\n", h[:])
	return h, nil
}

func (t *BencodeInfo) SplitPiecesHash() ([][20]byte, error) {
	hashLen := 20
	buf := []byte(t.Pieces)
	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("blah %d", len(buf))
		return nil, err
	}

	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}
	return hashes, nil
}

func (bto *BencodeTorrent) ToTorrentFile() (TorrentFile, error) {
	infoHash, err := bto.Info.Hash()
	if err != nil {
		return TorrentFile{}, err
	}
	piecesHash, err := bto.Info.SplitPiecesHash()
	if err != nil {
		return TorrentFile{}, err
	}

	length := 0
	if bto.Info.Length != 0 {
		length += bto.Info.Length
	} else {
		for _, file := range bto.Info.Files {
			length += file.Length
		}
	}

	t := TorrentFile{
		Announce:     bto.Announce,
		AnnounceList: bto.AnnounceList,
		InfoHash:     infoHash,
		PiecesHash:   piecesHash,
		Name:         bto.Info.Name,
		Files:        bto.Info.Files,
		PieceLength:  bto.Info.PieceLength,
		Length:       length,
		PiecesNum:    length / bto.Info.PieceLength,
	}
	return t, nil
}
