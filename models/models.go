package models

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
	"rcotillo.tech/torrent_downloader/commons"
)

type Torrent struct {
	Peers       []Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

// Port to listen on
const Port uint16 = 6881

type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
}

type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// Open parses a torrent file
func Open(r io.Reader) (*bencodeTorrent, error) {
	bto := bencodeTorrent{}
	err := bencode.Unmarshal(r, &bto)
	if err != nil {
		return nil, err
	}
	return &bto, nil
}

type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

func (t *TorrentFile) RequestPeers(peerID [20]byte, port uint16) ([]Peer, error) {
	trackers := commons.Trackers
	apeers := []Peer{}
	for _, tr := range trackers {
		url, err := t.buildTrackerURL(peerID, port, tr)

		c := &http.Client{Timeout: 15 * time.Second}
		resp, err := c.Get(url)
		if err != nil {
			resp.Body.Close()
			continue
		}
		defer resp.Body.Close()
		trackerResp := bencodeTrackerResp{}
		err = bencode.Unmarshal(resp.Body, &trackerResp)
		if err != nil {
			return nil, err
		}

		peers, err := Unmarshal([]byte(trackerResp.Peers))
		if err != nil {
			return nil, err
		}
		for _, p := range peers {
			apeers = append(apeers, p)
		}
	}
	return apeers, nil
}

func (bto bencodeTorrent) ToTorrentFile() (TorrentFile, error) {
	infoHash, err := bto.Info.hash()
	if err != nil {
		return TorrentFile{}, err
	}
	pieceHashes, err := bto.Info.splitPieceHashes()
	if err != nil {
		return TorrentFile{}, err
	}
	t := TorrentFile{
		Announce:    bto.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: bto.Info.PieceLength,
		Length:      bto.Info.Length,
		Name:        bto.Info.Name,
	}
	return t, nil
}

func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	hashLen := 20 // Length of SHA-1 hash
	buf := []byte(i.Pieces)
	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("Received malformed pieces of length %d", len(buf))
		return nil, err
	}
	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}
	return hashes, nil
}

func (t *TorrentFile) buildTrackerURL(peerID [20]byte, port uint16, d string) (string, error) {
	base, err := url.Parse(d)
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

// Peer encodes connection information for a peer
type Peer struct {
	IP   net.IP
	Port uint16
}

// Unmarshal parses peer IP addresses and ports from a buffer
func Unmarshal(peersBin []byte) ([]Peer, error) {
	const peerSize = 6 // 4 for IP, 2 for port
	numPeers := len(peersBin) / peerSize
	if len(peersBin)%peerSize != 0 {
		err := fmt.Errorf("Received malformed peers")
		return nil, err
	}
	peers := make([]Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16(peersBin[offset+4 : offset+6])
	}
	return peers, nil
}

// A Handshake is a special message that a peer uses to identify itself
type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

// Serialize serializes the handshake to a buffer
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49)
	buf[0] = byte(len(h.Pstr))
	curr := 1
	curr += copy(buf[curr:], h.Pstr)
	curr += copy(buf[curr:], make([]byte, 8)) // 8 reserved bytes
	curr += copy(buf[curr:], h.InfoHash[:])
	curr += copy(buf[curr:], h.PeerID[:])
	return buf
}

type Response struct {
	Data          Detail `json:"data"`
	Status        string `json:"status"`
	StatusMessage string `json:"status_message"`
}

type Detail struct {
	Movie Movie `json:"movie"`
}
type Movie struct {
	ID                      int           `json:"id"`
	ImdbCode                string        `json:"imdb_code"`
	Title                   string        `json:"title"`
	TitleEnglish            string        `json:"title_english"`
	TitleLong               string        `json:"title_long"`
	Slug                    string        `json:"slug"`
	Year                    int           `json:"year"`
	Rating                  float64       `json:"rating"`
	Runtime                 int           `json:"runtime"`
	Genres                  []interface{} `json:"genres"`
	Summary                 string        `json:"summary"`
	DescriptionFull         string        `json:"description_full"`
	Synopsis                string        `json:"synopsis"`
	YtTrailerCode           string        `json:"yt_trailer_code"`
	Language                string        `json:"language"`
	MpaRating               string        `json:"mpa_rating"`
	BackgroundImage         string        `json:"background_image"`
	BackgroundImageOriginal string        `json:"background_image_original"`
	SmallCoverImage         string        `json:"small_cover_image"`
	MediumCoverImage        string        `json:"medium_cover_image"`
	LargeCoverImage         string        `json:"larger_cover_image"`
	State                   string        `json:"State"`
	Torrents                []TorrentInfo `json:"torrents"`
	DateUploaded            string        `json:"date_uploaded"`
	DateUploadedUnix        int           `json:"date_uploaded_unix"`
}

type TorrentInfo struct {
	Url              string `json:"url"`
	Hash             string `json:"hash"`
	Quality          string `json:"quality"`
	Type             string `json:"type"`
	Seeds            int    `json:"seeds"`
	Peers            int    `json:"peers"`
	Size             string `json:"size"`
	SizeBytes        int    `json:"size_bytes"`
	DateUploaded     string `json:"date_uploaded"`
	DateUploadedUnix int    `json:"date_uploaded_unix"`
}

func getUDPAddr(d string) (net.UDPAddr, error) {
	h, p, err := net.SplitHostPort(d[6 : len(d)-9])
	if err != nil {
		return net.UDPAddr{}, err
	}
	ips, err := net.LookupIP(h)
	if err != nil {
		return net.UDPAddr{}, err
	}
	ip := ips[0]
	port, err := strconv.Atoi(p)
	if err != nil {
		return net.UDPAddr{}, err
	}
	return net.UDPAddr{IP: ip, Port: port}, nil
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

type Client struct {
	Conn     net.Conn
	Choked   bool
	peer     Peer
	infoHash [20]byte
	peerID   [20]byte
}
