package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

	var mr Response
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(b, &mr)

	if err != nil {
		panic(err)
	}
	t := mr.Data.Movie.Torrents[0]

	r, err = http.Get(t.Url)

	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	bt, err := Open(r.Body)

	if err != nil {
		panic(err)
	}
	tf, err := bt.toTorrentFile()

	if err != nil {
		panic(err)
	}

	var peerID [20]byte
	_, err = rand.Read(peerID[:])
	if err != nil {
		panic(err)
	}
	peers, err := tf.requestPeers(peerID, Port)

	if err != nil {
		panic(err)
	}
	fmt.Print(peers)

}
