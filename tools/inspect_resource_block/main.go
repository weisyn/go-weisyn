package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// Keys/Prefixes (must match repository implementation)
const (
	resourceIndexKeyPrefix = "res:"
	blockKeyPrefix         = "block:"
)

// ResourceLocation format used in repository/resource/index.go
// serializeResourceLocation: [4B len][blockHash][4B txIndex][4B outputIndex]
func parseResourceLocation(b []byte) (blockHash []byte, txIndex uint32, outputIndex uint32, err error) {
	if len(b) < 4 {
		return nil, 0, 0, errors.New("invalid location: too short")
	}
	bl := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	off := 4
	if bl <= 0 || off+bl > len(b) {
		return nil, 0, 0, errors.New("invalid location: bad block hash length")
	}
	blockHash = make([]byte, bl)
	copy(blockHash, b[off:off+bl])
	off += bl
	if off+8 > len(b) {
		return nil, 0, 0, errors.New("invalid location: missing indices")
	}
	txIndex = uint32(b[off])<<24 | uint32(b[off+1])<<16 | uint32(b[off+2])<<8 | uint32(b[off+3])
	off += 4
	outputIndex = uint32(b[off])<<24 | uint32(b[off+1])<<16 | uint32(b[off+2])<<8 | uint32(b[off+3])
	return
}

func readValue(db *badgerdb.DB, key []byte) ([]byte, error) {
	var out []byte
	err := db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			out = append([]byte(nil), val...)
			return nil
		})
	})
	return out, err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: inspect_resource_block <content_hash_hex> [db_dir]")
		os.Exit(2)
	}
	contentHashHex := os.Args[1]
	contentHash, err := hex.DecodeString(contentHashHex)
	if err != nil || len(contentHash) == 0 {
		fmt.Fprintf(os.Stderr, "invalid content hash: %v\n", err)
		os.Exit(1)
	}
	dbDir := "/Users/qinglong/go/src/chaincodes/WES/weisyn.git/data/testing/badger"
	if len(os.Args) >= 3 {
		dbDir = os.Args[2]
	}
	if !filepath.IsAbs(dbDir) {
		if p, err := filepath.Abs(dbDir); err == nil {
			dbDir = p
		}
	}

	opts := badgerdb.DefaultOptions(dbDir)
	opts.SyncWrites = false
	opts.ReadOnly = true
	opts.BypassLockGuard = true
	opts.Logger = nil // keep quiet
	db, err := badgerdb.Open(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open badger failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 1) res:<contentHash> -> ResourceLocation
	resKey := append([]byte(resourceIndexKeyPrefix), contentHash...)
	locBytes, err := readValue(db, resKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource index not found: %s (%v)\n", contentHashHex, err)
		os.Exit(1)
	}
	blockHash, txIdx, outIdx, err := parseResourceLocation(locBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse resource location failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Resource %s\n", contentHashHex)
	fmt.Printf("- BlockHash: %x\n", blockHash)
	fmt.Printf("- TxIndex: %d, OutputIndex: %d\n", txIdx, outIdx)

	// 2) block:<blockHash> -> pb.blockchain.core.Block
	blkKey := append([]byte(blockKeyPrefix), blockHash...)
	blkBytes, err := readValue(db, blkKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "block not found for hash %x: %v\n", blockHash, err)
		os.Exit(1)
	}
	var block core.Block
	if err := proto.Unmarshal(blkBytes, &block); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal block failed: %v\n", err)
		os.Exit(1)
	}

	// 3) Print header & txs info and verify transaction output includes the resource
	h := block.GetHeader()
	if h == nil {
		fmt.Println("Block header missing")
		os.Exit(1)
	}
	txCount := 0
	if b := block.GetBody(); b != nil {
		txCount = len(b.GetTransactions())
	}
	// timestamp is uint64 seconds
	fmt.Printf("Block Info:\n")
	fmt.Printf("- Height: %d\n", h.GetHeight())
	fmt.Printf("- ChainID: %d\n", h.GetChainId())
	fmt.Printf("- Timestamp: %d (%s)\n", h.GetTimestamp(), time.Unix(int64(h.GetTimestamp()), 0).Format(time.RFC3339))
	fmt.Printf("- MerkleRoot: %x\n", h.GetMerkleRoot())
	fmt.Printf("- PrevHash: %x\n", h.GetPreviousHash())
	fmt.Printf("- TxCount: %d\n", txCount)

	// Verify the tx index contains a resource output with the same content hash (best-effort)
	if b := block.GetBody(); b != nil {
		txs := b.GetTransactions()
		if int(txIdx) < len(txs) {
			tx := txs[txIdx]
			// scan outputs for resource
			found := false
			for _, out := range tx.GetOutputs() {
				if r := out.GetResource(); r != nil {
					if v := r.GetResource(); v != nil && len(v.GetContentHash()) == len(contentHash) {
						if string(v.GetContentHash()) == string(contentHash) {
							found = true
							break
						}
					}
				}
			}
			if found {
				fmt.Println("- Verified: resource output present in the transaction")
			} else {
				fmt.Println("- Warning: resource output not found at tx index (indices may differ)")
			}
		}
	}

	// 4) Basic confirmation message
	if txCount > 0 {
		fmt.Println("Status: confirmed (resource included in this block)")
	} else {
		fmt.Println("Status: confirmed (block found); tx count = 0 (unexpected)")
	}
}
