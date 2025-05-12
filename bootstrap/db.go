package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Database interface {
	StoreData(messageID string, data []interface{}, dataStructure []string, dataStructureMeta []string, timestamp int64, dataStructureID int) error
	StoreSignature(hash, signer, signature string) error
	GetData(hash string) ([]interface{}, []string, []string, int64, bool)
	GetSignatures(hash string) (map[string]string, bool)
	GetAllMessages(dataStructureID int, page, limit int) ([]Message, error)
	GetLatestMessage(dataStructureID int) (Message, bool, error)
	GetMessagesByField(dataStructureID int, field, value string, page, limit int) ([]Message, error)
	GetLatestByField(dataStructureID, threshold int, field, value string) (Message, bool, error)
	GetDataStructures() ([]int, error)
	GetDataStructureStats(id, threshold int) (DataStructureStats, error)
	Close() error
}

type Message struct {
	Hash              string            `json:"hash"`
	Data              []interface{}     `json:"data"`
	DataStructure     []string          `json:"data_structure"`
	DataStructureMeta []string          `json:"data_structure_meta"`
	Signatures        map[string]string `json:"signatures"`
	Timestamp         int64             `json:"timestamp"`
}

type DataStructureStats struct {
	ID                int    `json:"id"`
	MessageCount      int    `json:"message_count"`
	LastMessageTime   int64  `json:"last_message_time"`
	LastConfirmedTime int64  `json:"last_confirmed_time"`
	LastConfirmedHash string `json:"last_confirmed_hash"`
}

type LevelDBDatabase struct {
	db   *leveldb.DB
	mu   sync.RWMutex
	path string
}

func NewLevelDBDatabase(path string) (*LevelDBDatabase, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %w", err)
	}

	return &LevelDBDatabase{
		db:   db,
		path: path,
	}, nil
}

const (
	dataPrefix       = "data:"
	signaturePrefix  = "sig:"
	trustedPrefix    = "trusted:"
	dataStructPrefix = "ds:"
	indexPrefix      = "index:"
)

func (ldb *LevelDBDatabase) Close() error {
	return ldb.db.Close()
}

func (ldb *LevelDBDatabase) StoreData(hash string, data []interface{}, dataStructure []string, dataStructureMeta []string, timestamp int64, dataStructureID int) error {
	ldb.mu.Lock()
	defer ldb.mu.Unlock()

	dataMap := make(map[string]interface{})
	for i, field := range dataStructureMeta {
		if i < len(data) {
			dataMap[field] = data[i]
		}
	}

	msg := Message{
		Hash:              hash,
		Data:              data,
		DataStructure:     dataStructure,
		DataStructureMeta: dataStructureMeta,
		Timestamp:         timestamp,
	}

	dsKey := []byte(dataStructPrefix + fmt.Sprintf("%d", dataStructureID))
	if exists, _ := ldb.db.Has(dsKey, nil); !exists {
		dsData, err := json.Marshal(dataStructure)
		if err != nil {
			return fmt.Errorf("failed to marshal data structure: %w", err)
		}
		if err := ldb.db.Put(dsKey, dsData, nil); err != nil {
			return fmt.Errorf("failed to store data structure: %w", err)
		}
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Store by hash with data structure ID reference
	if err := ldb.db.Put([]byte(dataPrefix+hash), msgData, nil); err != nil {
		return fmt.Errorf("failed to store message by hash: %w", err)
	}

	// Create timestamp index with data structure ID
	indexKey := []byte(fmt.Sprintf("%s%d:%d:%s", indexPrefix, dataStructureID, timestamp, hash))
	if err := ldb.db.Put(indexKey, []byte{}, nil); err != nil {
		return fmt.Errorf("failed to create timestamp index: %w", err)
	}

	// Create field indexes with data structure ID
	for field, value := range dataMap {
		fieldIndexKey := []byte(fmt.Sprintf("%s%d:%s:%v:%s", indexPrefix, dataStructureID, field, value, hash))
		if err := ldb.db.Put(fieldIndexKey, []byte{}, nil); err != nil {
			return fmt.Errorf("failed to create field index: %w", err)
		}
	}

	return nil
}

func (ldb *LevelDBDatabase) StoreSignature(hash, signer, signature string) error {
	ldb.mu.Lock()
	defer ldb.mu.Unlock()

	sigKey := []byte(signaturePrefix + hash)
	var sigs map[string]string

	if sigData, err := ldb.db.Get(sigKey, nil); err == nil {
		if err := json.Unmarshal(sigData, &sigs); err != nil {
			return fmt.Errorf("failed to unmarshal signatures: %w", err)
		}
	} else if err != leveldb.ErrNotFound {
		return fmt.Errorf("failed to get signatures: %w", err)
	} else {
		sigs = make(map[string]string)
	}

	sigs[signer] = signature

	sigData, err := json.Marshal(sigs)
	if err != nil {
		return fmt.Errorf("failed to marshal signatures: %w", err)
	}

	if err := ldb.db.Put(sigKey, sigData, nil); err != nil {
		return fmt.Errorf("failed to store signatures: %w", err)
	}

	return nil
}

func (ldb *LevelDBDatabase) GetData(hash string) ([]interface{}, []string, []string, int64, bool) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	data, err := ldb.db.Get([]byte(dataPrefix+hash), nil)
	if err != nil {
		return nil, nil, nil, 0, false
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, nil, nil, 0, false
	}

	sigs, exists := ldb.GetSignatures(hash)
	if exists {
		msg.Signatures = sigs
	}

	return msg.Data, msg.DataStructure, msg.DataStructureMeta, msg.Timestamp, true
}

func (ldb *LevelDBDatabase) GetSignatures(hash string) (map[string]string, bool) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	sigData, err := ldb.db.Get([]byte(signaturePrefix+hash), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return make(map[string]string), false
		}
		return nil, false
	}

	var sigs map[string]string
	if err := json.Unmarshal(sigData, &sigs); err != nil {
		return nil, false
	}

	return sigs, true
}

func (ldb *LevelDBDatabase) GetAllMessages(dataStructureID int, page, limit int) ([]Message, error) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	var messages []Message

	prefix := []byte(fmt.Sprintf("%s%d:", indexPrefix, dataStructureID))
	iter := ldb.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	// Calculate how many records to skip
	skip := (page - 1) * limit
	count := 0

	// Iterate from newest to oldest (Last to Prev)
	for iter.Last(); iter.Valid(); iter.Prev() {

		key := string(iter.Key())
		parts := strings.Split(key, ":")
		if len(parts) < 4 {
			continue
		}
		hash := parts[3]

		data, err := ldb.db.Get([]byte(dataPrefix+hash), nil)
		if err != nil {
			continue
		}

		if skip > 0 {
			skip--
			continue
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		sigs, exists := ldb.GetSignatures(msg.Hash)
		if exists {
			msg.Signatures = sigs
		}

		messages = append(messages, msg)
		count++

		if count >= limit {
			break
		}
	}

	return messages, nil
}

func (ldb *LevelDBDatabase) GetLatestMessage(dataStructureID int) (Message, bool, error) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	var prefix []byte
	prefix = []byte(fmt.Sprintf("%s%d:", indexPrefix, dataStructureID))

	iter := ldb.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	if !iter.Last() {
		return Message{}, false, leveldb.ErrNotFound
	}

	key := string(iter.Key())
	var hash string

	parts := strings.Split(key, ":")
	if len(parts) < 4 {
		return Message{}, false, fmt.Errorf("invalid index key format")
	}
	hash = parts[3]

	data, err := ldb.db.Get([]byte(dataPrefix+hash), nil)
	if err != nil {
		return Message{}, false, err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return Message{}, false, err
	}

	sigs, exists := ldb.GetSignatures(msg.Hash)
	if exists {
		msg.Signatures = sigs
		return msg, true, nil
	}

	return msg, false, nil
}

func (ldb *LevelDBDatabase) GetMessagesByField(dataStructureID int, field, value string, page, limit int) ([]Message, error) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	var messages []Message

	prefix := []byte(fmt.Sprintf("%s%d:%s:%v:", indexPrefix, dataStructureID, field, value))
	iter := ldb.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	skipped := 0
	for iter.Next() {
		key := string(iter.Key())
		messageID := key[len(prefix):]

		data, err := ldb.db.Get([]byte(dataPrefix+messageID), nil)
		if err != nil {
			continue
		}

		if skipped < page*limit {
			skipped++
			continue
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		sigs, exists := ldb.GetSignatures(msg.Hash)
		if exists {
			msg.Signatures = sigs
		}

		messages = append(messages, msg)

		if len(messages) >= limit {
			break
		}
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp > messages[j].Timestamp
	})

	return messages, nil
}

func (ldb *LevelDBDatabase) GetLatestByField(dataStructureID, threshold int, field, value string) (Message, bool, error) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	prefix := []byte(fmt.Sprintf("%s%d:%s:%v:", indexPrefix, dataStructureID, field, value))
	iter := ldb.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	var latest Message
	found := false

	for iter.Next() {
		key := string(iter.Key())
		messageID := key[len(prefix):]

		data, err := ldb.db.Get([]byte(dataPrefix+messageID), nil)
		if err != nil {
			continue
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		sigs, exists := ldb.GetSignatures(msg.Hash)
		if exists && len(sigs) >= threshold {
			if !found || msg.Timestamp > latest.Timestamp {
				msg.Signatures = sigs
				latest = msg
				found = true
			}
		}
	}

	return latest, found, nil
}

func (ldb *LevelDBDatabase) GetDataStructures() ([]int, error) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	var ids []int
	iter := ldb.db.NewIterator(util.BytesPrefix([]byte(dataStructPrefix)), nil)
	defer iter.Release()

	for iter.Next() {
		key := string(iter.Key())
		idStr := strings.TrimPrefix(key, dataStructPrefix)
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func (ldb *LevelDBDatabase) GetDataStructureStats(id, threshold int) (DataStructureStats, error) {
	ldb.mu.RLock()
	defer ldb.mu.RUnlock()

	stats := DataStructureStats{ID: id}
	prefix := []byte(fmt.Sprintf("%s%d:", indexPrefix, id))

	iter := ldb.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	for iter.Next() {
		stats.MessageCount++

		key := string(iter.Key())
		parts := strings.Split(key, ":")
		if len(parts) < 4 {
			continue
		}

		timestamp, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			continue
		}

		if timestamp > stats.LastMessageTime {
			stats.LastMessageTime = timestamp
		}

		hash := parts[3]
		if sigs, exists := ldb.GetSignatures(hash); exists && len(sigs) >= threshold {
			if timestamp > stats.LastConfirmedTime {
				stats.LastConfirmedTime = timestamp
				stats.LastConfirmedHash = hash
			}
		}
	}

	return stats, nil
}
