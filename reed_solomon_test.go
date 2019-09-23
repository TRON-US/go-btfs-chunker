package chunk

import (
	"bytes"
	"io"
	"testing"
	"time"

	rs "github.com/klauspost/reedsolomon"
)

const (
	testRsDefaultNumData   = 20
	testRsDefaultNumParity = 10
	testRsDefaultSize      = 1024 * 256
)

func TestReedSolomonSplitterSplitMerge(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	max := 10000000
	b := randBuf(t, max)
	// Create reference split shards
	enc, err := rs.New(testRsDefaultNumData, testRsDefaultNumParity)
	if err != nil {
		t.Fatal("unable to create reference reedsolomon object", err)
	}
	shards, err := enc.Split(b)
	if err != nil {
		t.Fatal("unable to split reference reedsolomon shards", err)
	}
	err = enc.Encode(shards)
	if err != nil {
		t.Fatal("unable to encode reference reedsolomon shards", err)
	}
	// Make sure we can reconstruct
	var joined bytes.Buffer
	err = enc.Join(io.Writer(&joined), shards, max)
	if err != nil {
		t.Fatal("unable to reference join original file", err)
	}
	if !bytes.Equal(joined.Bytes(), b) {
		t.Fatal("joined file does not match original")
	}
	// Length for each data shard
	shardLen := len(shards[0])

	// Splitter reader
	r := &clipReader{r: bytes.NewReader(b), size: 4000}
	spl, err := NewReedSolomonSplitter(r,
		testRsDefaultNumData, testRsDefaultNumParity, testRsDefaultSize)
	if err != nil {
		t.Fatal("unable to create reed solomon splitter", err)
	}

	c, errc := Chan(spl)

	for sn := 0; sn < testRsDefaultNumData+testRsDefaultNumParity; sn++ {
		var shard []byte
		// Combine expected number of chunks to create shard
		for cn := 0; cn < (shardLen+testRsDefaultSize-1)/testRsDefaultSize; cn++ {
			select {
			case chunk := <-c:
				if len(chunk) > testRsDefaultSize {
					t.Fatalf("splitter returns a larger than expected size (chunk number %d): %d",
						cn, len(chunk))
				}
				shard = append(shard, chunk...)
				// Smaller chunk must be at the end of shard
				if len(chunk) < testRsDefaultSize {
					break
				}
				// continue
			case err := <-errc:
				t.Fatal("failed to split all chunks", err)
			case <-time.After(5 * time.Second):
				t.Fatal("timed out while waiting for chunk split")
			}
		}

		// The initial testRsDefaultNumData shards are the data shards
		// The rest testRsDefaultNumParity shards are the parity shards,
		// verified against a normal reed-solomon split
		bs := shards[sn]
		if !bytes.Equal(bs, shard) {
			t.Fatalf("shard not correct: (shard number: %d) %d != %d, %v != %v",
				sn, len(bs), len(shard), bs[:100], shard[:100])
		}
	}
}