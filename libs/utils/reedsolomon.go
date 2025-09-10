package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/klauspost/reedsolomon"
)

const (
	MiB           = 1024 * 1024
	SEGMENT_SIZE  = 32 * MiB
	FRAGMENT_SIZE = 8 * MiB
	FRAGMENTS_NUM = 4
	PARITY_NUM    = 8
)

func ReedSolomon(file string, saveDir string) ([]string, error) {
	var shardspath = make([]string, 0)
	fstat, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	if fstat.Size() != SEGMENT_SIZE {
		return nil, errors.New("invalid size")
	}

	enc, err := reedsolomon.New(FRAGMENTS_NUM, PARITY_NUM)
	if err != nil {
		return shardspath, err
	}

	b, err := os.ReadFile(file)
	if err != nil {
		return shardspath, err
	}

	shards, err := enc.Split(b)
	if err != nil {
		return shardspath, err
	}

	err = enc.Encode(shards)
	if err != nil {
		return shardspath, err
	}

	for _, shard := range shards {
		hash, err := CalcSHA256(shard)
		if err != nil {
			return shardspath, err
		}
		newpath := filepath.Join(saveDir, hash)
		err = os.WriteFile(newpath, shard, 0755)
		if err != nil {
			return shardspath, err
		}
		shardspath = append(shardspath, newpath)
	}
	return shardspath, nil
}

func ReedSolomonWithHandle(file []byte, handle func([]byte) error) error {
	enc, err := reedsolomon.New(FRAGMENTS_NUM, PARITY_NUM)
	if err != nil {
		return err
	}
	shards, err := enc.Split(file)
	if err != nil {
		return err
	}

	if err = enc.Encode(shards); err != nil {
		return err
	}

	for _, shard := range shards {
		if err = handle(shard); err != nil {
			return err
		}
	}
	shards = nil
	return nil
}

func RSRestore(outpath string, shardspath []string) error {
	enc, err := reedsolomon.New(FRAGMENTS_NUM, PARITY_NUM)
	if err != nil {
		return err
	}

	shards := make([][]byte, FRAGMENTS_NUM+PARITY_NUM)
	count := 0
	for k, v := range shardspath {
		if count >= FRAGMENTS_NUM {
			break
		}
		shards[k], err = os.ReadFile(v)
		if err != nil {
			shards[k] = nil
		} else {
			count++
		}

	}

	// Verify the shards
	ok, _ := enc.Verify(shards)
	if !ok {
		err = enc.Reconstruct(shards)
		if err != nil {
			return err
		}
		ok, err = enc.Verify(shards)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invalid shards")
		}
	}

	f, err := os.Create(outpath)
	if err != nil {
		return err
	}
	defer f.Close()
	err = enc.Join(f, shards, len(shards[0])*FRAGMENTS_NUM)
	return err
}
