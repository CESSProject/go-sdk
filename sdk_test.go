package sdkgo_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CESSProject/go-sdk/chain"
	"github.com/CESSProject/go-sdk/retriever"
	"github.com/centrifuge/go-substrate-rpc-client/v4/registry"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/klauspost/reedsolomon"
	"github.com/panjf2000/ants"
	"github.com/pkg/errors"
)

var (
	ErrorNotFound = errors.New("not found")
)

func TestErrorWarp(t *testing.T) {
	err := errors.Wrap(ErrorNotFound, "test error warp")
	t.Log(errors.Unwrap(errors.Unwrap(err)))
}

func TestTransfer(t *testing.T) {
	//
	//skill income exile ethics sick excess sea deliver medal junk update fault
	cli, err := chain.NewLightCessClient(
		"concert ostrich mass worry powder traffic clinic beauty travel suggest satoshi outside",
		[]string{"wss://t2-rpc.cess.network"},
	)
	if err != nil {
		t.Fatal(err)
	}
	total, errCount := 500, &atomic.Int32{}
	wg := sync.WaitGroup{}
	wg.Add(total)
	st := time.Now()
	pool, err := ants.NewPool(500)
	if err != nil {
		t.Fatal(err)
	}
	for i := range total {
		idx := i
		pool.Submit(func() {
			defer wg.Done()
			tx, err := cli.TransferToken("cXkGyoXtxnK2Zbw8X5gArXi9VGqKqE7b517muih45ds9Ebdno", "1000000000000000000", nil, nil)
			if err != nil {
				t.Log(err)
				errCount.Add(1)
				return
			}
			t.Log(idx, "success,block hash:", tx)
		})
	}
	wg.Wait()
	t.Log("time:", time.Since(st), "total:", total, "error:", errCount.Load())
}

func TestUplaodWithPre(t *testing.T) {

	baseUrl := "https://retriever.cess.network"
	territory := "test1"
	filename := "test_random_file"
	mnemonic := "wing horse perfect monkey build squirrel embrace jacket frost make know save"
	keyPair, err := signature.KeyringPairFromSecret(mnemonic, 0)
	if err != nil {
		t.Fatal(err)
	}
	message := fmt.Sprint(time.Now().Unix())
	sign, err := retriever.SignedSR25519WithMnemonic(mnemonic, []byte(message))
	if err != nil {
		t.Fatal(err)
	}
	acc := chain.EncodePubkey(keyPair.PublicKey, 11330)
	token, err := retriever.GenGatewayAccessToken(baseUrl, message, acc, sign, 0)
	if err != nil {
		t.Fatal(err)
	}
	st := time.Now()
	buf := make([]byte, 1024*1024*129)
	if _, err = rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("./source_file")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.Write(buf); err != nil {
		t.Fatal(err)
	}
	t.Log("gen random time", time.Since(st))
	st = time.Now()
	fid, err := retriever.UploadFile(baseUrl, token, territory, filename, bytes.NewBuffer(buf), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("fid:", fid, "spend time:", time.Since(st))
}

func TestGetCapsuleAndDownloadData(t *testing.T) {
	baseUrl := "https://retriever.cess.network"
	fid := "704db5a38548c13ef23ff465622e474354acd2ccfd32f0313cb33e3cf3f8a652" //704db5a38548c13ef23ff465622e474354acd2ccfd32f0313cb33e3cf3f8a652
	mnemonic := "wing horse perfect monkey build squirrel embrace jacket frost make know save"

	capsule, pubkey, err := retriever.GetPreCapsuleAndGatewayPubkey(baseUrl, fid)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("capsule:", string(capsule))
	t.Log("gateway pubkey:", pubkey)
	rk, pkX, err := retriever.GenReEncryptionKey(mnemonic, pubkey)
	if err != nil {
		t.Fatal(err)
	}
	err = retriever.DownloadData(baseUrl, fid, "", "./rand_file", capsule, rk, pkX)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("success")
}

func TestSdkEventMap(t *testing.T) {
	cli, err := chain.NewLightCessClient(
		"wing horse perfect monkey build squirrel embrace jacket frost make know save",
		[]string{"wss://t2-rpc.cess.network"},
	)
	if err != nil {
		t.Fatal(err)
	}

	factory := registry.NewFactory()
	eventRegistry, err := factory.CreateEventRegistry(cli.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	eventMap := make(map[string][]string)
	for _, event := range eventRegistry {
		s := strings.Split(event.Name, ".")
		if events, ok := eventMap[s[0]]; ok {
			events = append(events, event.Name)
			eventMap[s[0]] = events
		} else {
			eventMap[s[0]] = []string{event.Name}
		}
	}
	for k, v := range eventMap {
		t.Log(k)
		for _, event := range v {
			t.Log("    ", event)
		}
	}
}

func TestSdkEvents(t *testing.T) {
	cli, err := chain.NewLightCessClient(
		"wing horse perfect monkey build squirrel embrace jacket frost make know save",
		[]string{"wss://t2-rpc.cess.network"},
	)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := cli.GetConnectionClient()
	if err != nil {
		t.Fatal(err)
	}
	sub, err := conn.RPC.Chain.SubscribeNewHeads()
	if err != nil {
		t.Fatal(err)
	}
	// callRegistry, err := registry.NewFactory().CreateCallRegistry(cli.Metadata)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	defer sub.Unsubscribe()
	for {
		select {
		case header := <-sub.Chan():
			hash, err := conn.RPC.Chain.GetBlockHash(uint64(header.Number))
			if err != nil {
				t.Log(err)
				continue
			}
			data, err := cli.ParseBlockData(hash)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("parse block ", hash)
			for _, ext := range data.Extrinsics {
				jb, err := json.Marshal(ext.Events)
				if err != nil {
					t.Fatal(err)
				}
				t.Log(ext.Name, "extrinsic events", string(jb))
			}
			jb, err := json.Marshal(data.SystemEvents)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("system events", string(jb))
			// block, err := cli.SubstrateAPI.RPC.Chain.GetBlock(hash)
			// if err != nil {
			// 	t.Log(err)
			// 	continue
			// }
			// t.Log(hash.Hex(), "Block Extrinsics")
			// for _, e := range block.Block.Extrinsics {
			// 	data, err := codec.Encode(e.Method)
			// 	if err != nil {
			// 		t.Log("encode error", err)
			// 	}
			// 	h := blake2b.Sum256(data)
			// 	call := callRegistry[e.Method.CallIndex]
			// 	t.Log(call.Name, e.Signature.Signer.AsAddress32, hexutil.Encode(h[:]))
			// 	for _, field := range call.Fields {
			// 		t.Log("    ", field.Name)
			// 	}
			// }
			// events, err := cli.Retriever.GetEvents(hash)
			// if err != nil {
			// 	t.Log(err)
			// 	continue
			// }
			// t.Log(hash.Hex(), "Block Events")
			// for _, e := range events {
			// 	t.Log(e.Name)
			// }
		case err := <-sub.Err():
			t.Fatal(err)
		}
	}
}

func TestBatchUploadRequest(t *testing.T) {
	baseUrl := "http://154.194.34.195:1306"
	mnemonic := "skill income exile ethics sick excess sea deliver medal junk update fault"
	message := "123456"
	territory := "test1"
	account := "cXkGyoXtxnK2Zbw8X5gArXi9VGqKqE7b517muih45ds9Ebdno"
	sign, err := retriever.SignedSR25519WithMnemonic(mnemonic, []byte(message))
	if err != nil {
		t.Fatal(err)
	}
	token, err := retriever.GenGatewayAccessToken(baseUrl, message, account, sign, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("token", token)
	hash, err := retriever.RequestBatchUpload(baseUrl, token, territory, "testfile1", 4096*1024, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hash)
}

func TestBatchUploadFile(t *testing.T) {
	baseUrl := "http://154.194.34.195:1306"
	mnemonic := "skill income exile ethics sick excess sea deliver medal junk update fault"
	message := "123456"
	account := "cXkGyoXtxnK2Zbw8X5gArXi9VGqKqE7b517muih45ds9Ebdno"
	hash := "a8e2fd9a3e237efe3ae8411fb3e3562ec1570fbac88acd9160b18bd375674b67"
	sign, err := retriever.SignedSR25519WithMnemonic(mnemonic, []byte(message))
	if err != nil {
		t.Fatal(err)
	}
	token, err := retriever.GenGatewayAccessToken(baseUrl, message, account, sign, 0)
	if err != nil {
		t.Fatal(err)
	}
	writer := bytes.NewBuffer(nil)
	reader, err := GenRandomBlockData(writer, 4096*1024)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("./testfile1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(f, reader); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	for size := int64(512 * 1024); size < 4096*1024; size += 512 * 1024 {
		res, err := retriever.BatchUploadFile(baseUrl, token, hash, f, size, size+512*1024)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("result:", res)
	}
}

func TestUploadDataToGateway(t *testing.T) {
	baseUrl := "http://154.194.34.195:1306"
	mnemonic := "skill income exile ethics sick excess sea deliver medal junk update fault"
	message := "123456"
	territory := "test1"
	account := "cXkGyoXtxnK2Zbw8X5gArXi9VGqKqE7b517muih45ds9Ebdno"
	sign, err := retriever.SignedSR25519WithMnemonic(mnemonic, []byte(message))
	if err != nil {
		t.Fatal(err)
	}
	token, err := retriever.GenGatewayAccessToken(baseUrl, message, account, sign, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("token", token)
	wg := &sync.WaitGroup{}
	errCounter := &atomic.Int32{}
	total := 3000
	wg.Add(total)
	for i := range total {
		go func(i int) {
			defer wg.Done()
			writer := bytes.NewBuffer(nil)
			for range 1 {
				reader, err := GenRandomBlockData(writer, 4096)
				if err != nil {
					t.Log(err)
					return
				}
				st := time.Now()
				fhash, err := retriever.UploadFile(baseUrl, token, territory, fmt.Sprintf("test_file_%d", i+60000), reader, false)
				if err != nil {
					errCounter.Add(1)
					t.Log(err)
					return
				}
				t.Log("goroutine", i, fhash, time.Since(st))
			}
		}(i)
	}
	wg.Wait()
	t.Log("File upload stress test completed, total:", total, " err:", errCounter.Load())
}

func TestReedSolomon(t *testing.T) {
	enc, err := reedsolomon.New(4, 8)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32*1024*1024)
	if _, err = rand.Read(buf[:1024*1024]); err != nil {
		t.Fatal(err)
	}
	shards, err := enc.Split(buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("splited data shards:")
	for i, s := range shards {
		h := sha256.Sum256(s)
		t.Log(i, hex.EncodeToString(h[:]))
	}
	if err = enc.Encode(shards); err != nil {
		t.Fatal(err)
	}
	t.Log("encoded data shards:")
	for i, s := range shards {
		h := sha256.Sum256(s)
		t.Log(i, hex.EncodeToString(h[:]))
	}
	zeroData := make([]byte, 8*1024*1024)
	h := sha256.Sum256(zeroData)
	t.Log("zero value data:", hex.EncodeToString(h[:]))
}

func GenRandomBlockData(writer *bytes.Buffer, size int64) (io.Reader, error) {
	buf := make([]byte, 4096)
	writer.Reset()
	for written := int64(0); written < size; written += 4096 {
		var bytesToWrite int64 = 4096
		if written+bytesToWrite > size {
			bytesToWrite = size - written
		}
		_, err := rand.Read(buf[:bytesToWrite])
		if err != nil {
			return writer, err
		}
		writer.Write(buf[:bytesToWrite])
	}
	return writer, nil
}
