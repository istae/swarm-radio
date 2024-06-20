package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethersphere/bee/pkg/api"
	"github.com/ethersphere/bee/pkg/cac"
	"github.com/ethersphere/bee/pkg/crypto"
	"github.com/ethersphere/bee/pkg/soc"
	"github.com/ethersphere/bee/pkg/swarm"
)

const port = 9999
const topicStr = "test radio"

type segmentState struct {
	ref swarm.Address
	ts  time.Time
}

func (s *segmentStore) store(key string, value swarm.Address) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.segments[key] = segmentState{ref: value, ts: time.Now()}
}

func (s *segmentStore) get(key string) swarm.Address {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.segments[key].ref
}

func (s *segmentStore) cleanup() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for k, v := range s.segments {
		if time.Since(v.ts) > time.Hour {
			delete(s.segments, k)
		}
	}
}

type segmentStore struct {
	mtx      sync.Mutex
	segments map[string]segmentState
}

func main() {

	var (
		privateKey string
		batchID    string
		since      time.Duration
	)

	flag.StringVar(&privateKey, "private-key", "", "")
	flag.StringVar(&batchID, "batch-id", "", "")
	flag.DurationVar(&since, "since", time.Hour*24*7, "amount of time to rollback")
	flag.Parse()

	if privateKey == "" || batchID == "" {
		log.Fatal("missing flag")
	}

	mtx := sync.Mutex{}

	store := &segmentStore{segments: map[string]segmentState{}}

	var index atomic.Uint64
	index.Store(0)

	topicRaw, err := crypto.LegacyKeccak256([]byte(topicStr))
	if err != nil {
		log.Fatal(err)
	}
	topicStr := hex.EncodeToString(topicRaw[:])
	fmt.Println("topic", topicStr)

	privKeyRaw, _ := hex.DecodeString(privateKey)
	privKey, err := crypto.DecodeSecp256k1PrivateKey(privKeyRaw)
	if err != nil {
		log.Fatal(err)
	}

	signer := crypto.NewDefaultSigner(privKey)
	signer.EthereumAddress()
	ethAddr, err := signer.EthereumAddress()
	if err != nil {
		log.Fatal(err)
	}

	ethAddrStr := ethAddr.String()[2:]
	fmt.Println("owner", ethAddrStr)

	idx, err := getFeed(ethAddrStr, topicStr)
	if err != nil {
		log.Fatal(err)
	}

	if idx > 0 {
		index.Store(idx + 1)
	}

	fmt.Println("using index", index.Load())

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		mtx.Lock()
		defer mtx.Unlock()
		defer store.cleanup()

		_, file := path.Split(r.URL.Path)
		data, _ := io.ReadAll(r.Body)

		if strings.Contains(file, ".m3u8") {

			fmt.Println(string(data))

			manifest := emplaceM3u8Urls(string(data), store)
			fmt.Println("manifest", manifest)

			_, err := updateFeed(ethAddrStr, batchID, identifier(topicStr, index.Load()), []byte(manifest), signer)
			if err != nil {
				fmt.Println("error", err)
				return
			}

			index.Add(1)

		} else {
			ref, err := uploadData(data, batchID)
			if err != nil {
				fmt.Println("error", err)
				return
			}
			store.store(file, ref)
			fmt.Println("uploaded segment", ref)

		}

	}))

	fmt.Printf("Starting server on %v\n", port)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}

func emplaceM3u8Urls(manifest string, store *segmentStore) string {

	lines := strings.Split(manifest, "\n")

	for i, l := range lines {
		if strings.Contains(l, ".ts") {
			lines[i] = fmt.Sprintf("/bytes/%s", store.get(l).String())
		}
	}

	return strings.Join(lines, "\n")
}

type referenceResponse struct {
	Reference swarm.Address `json:"reference"`
}

func parseRef(r io.Reader) (swarm.Address, error) {

	data, err := io.ReadAll(r)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	var ref referenceResponse
	err = json.Unmarshal(data, &ref)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	return ref.Reference, nil
}

func getFeed(ethAddr, topic string) (uint64, error) {

	resp, err := http.Get(fmt.Sprintf("http://localhost:1633/feeds/%s/%s", ethAddr, topic))
	if err != nil {
		return 0, err
	}

	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("feed not found")
		return 0, nil
	}

	hexIndex := resp.Header.Get(api.SwarmFeedIndexHeader)
	indexRaw, err := hex.DecodeString(hexIndex)
	if err != nil {
		return 0, err
	}

	binary.BigEndian.Uint64(indexRaw)

	return binary.BigEndian.Uint64(indexRaw), nil
}

func updateFeed(owner string, batchID string, id []byte, data []byte, signer crypto.Signer) (swarm.Address, error) {

	// manifest data
	ref, err := uploadData(data, batchID)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	// prepare ref data for soc upload
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Unix()))
	data = append(buf, ref.Bytes()...)

	ch, err := cac.New(data)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	sch, err := soc.New(id, ch).Sign(signer)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	fmt.Println("soc", sch.Address())

	sigStr := hex.EncodeToString(sch.Data()[swarm.HashSize : swarm.HashSize+swarm.SocSignatureSize])
	idStr := hex.EncodeToString(id)

	// upload soc with topic and id
	r, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:1633/soc/%s/%s?sig=%s", owner, idStr, sigStr), bytes.NewBuffer(ch.Data()))
	if err != nil {
		return swarm.ZeroAddress, err
	}
	r.Header.Set(api.SwarmPostageBatchIdHeader, batchID)
	r.Header.Set("content-type", "application/octet-stream")

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	return parseRef(res.Body)
}

func uploadData(data []byte, batchID string) (swarm.Address, error) {

	// upload data and get swarm reference
	r, err := http.NewRequest("POST", "http://localhost:1633/bytes", bytes.NewBuffer(data))
	if err != nil {
		return swarm.ZeroAddress, err
	}
	r.Header.Set(api.SwarmPostageBatchIdHeader, batchID)

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return swarm.ZeroAddress, err
	}

	return parseRef(resp.Body)
}

func identifier(topic string, index uint64) []byte {

	indexB := make([]byte, 8)
	binary.BigEndian.PutUint64(indexB, index)

	topicHex, _ := hex.DecodeString(topic)

	h, err := crypto.LegacyKeccak256(append(topicHex, indexB...))
	if err != nil {
		log.Fatal(err)
	}
	return h

}
