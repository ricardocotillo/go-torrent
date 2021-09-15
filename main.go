package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"

	"rcotillo.tech/torrent_downloader/models"
)

func main() {
	r, err := http.Get("https://yts.mx/api/v2/movie_details.json?movie_id=13106")

	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	b, err := ioutil.ReadAll(r.Body)

	if err != nil {
		panic(err)
	}

	var mr models.Response
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(b, &mr)

	if err != nil {
		panic(err)
	}
	t := mr.Data.Movie.Torrents[0]

	fmt.Println("this torrent has " + strconv.Itoa(t.Peers) + " peers")

	r, err = http.Get(t.Url)

	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	bt, err := models.Open(r.Body)

	if err != nil {
		panic(err)
	}
	tf, err := bt.ToTorrentFile()

	if err != nil {
		panic(err)
	}

	var peerID [20]byte
	_, err = rand.Read(peerID[:])
	if err != nil {
		panic(err)
	}
	peers, err := tf.RequestPeers(peerID, models.Port)

	if err != nil {
		panic(err)
	}
	hshake := buildHandcheck(tf, peerID)
	fmt.Println(peers)
	fmt.Println(len(hshake))
	for i, p := range peers {
		fmt.Println("connection to peer " + strconv.Itoa(i+1))
		conn, e := net.DialTimeout("tcp", p.String(), 15*time.Second)
		if e != nil {
			fmt.Println(e)
			continue
		}

		defer conn.Close()

		_, e = conn.Write(hshake)
		if e != nil {
			fmt.Println(e)
			continue
		}

		defer conn.Close()

		bdata := make([]byte, 1024)
		conn.Read(bdata)
		fmt.Println("data from peer " + strconv.Itoa(i+1))
		fmt.Println(bdata)
	}
}

func buildHandcheck(t models.TorrentFile, peerID [20]byte) []byte {
	pstr := []byte("BitTorrent protocol")
	pstrlen := len(pstr)
	buf := []byte{uint8(pstrlen)}
	buf = append(buf, pstr...)
	resrv := make([]byte, 8)
	buf = append(buf, resrv...)
	buf = append(buf, t.InfoHash[:]...)
	buf = append(buf, peerID[:]...)
	fmt.Println(buf)
	return buf
}
