package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/crypto/sha3"
)

type DataStructure struct {
	Fields []struct {
		Name         string `json:"name"`
		SolidityType string `json:"solidity_type"`
	} `json:"fields"`
}

type MessageBuilder interface {
	BuildMessage(price float64) (*SignRequest, error)
}

type StockQuoteMessageBuilder struct {
	Ticker           string
	StructureID      string
	DestinationChain int
	Structure        DataStructure
}

func SolidityKeccak256(types []string, values []interface{}) []byte {
	if len(types) != len(values) {
		panic("types and values length mismatch")
	}

	var packed []byte

	for i, typ := range types {
		switch typ {
		case "bytes32":
			val, ok := values[i].([32]byte)
			if !ok {
				panic("invalid bytes32 value")
			}
			packed = append(packed, val[:]...)

		case "string":
			val, ok := values[i].(string)
			if !ok {
				panic("invalid string value")
			}
			packed = append(packed, []byte(val)...)

		case "uint256":
			val, ok := values[i].(*big.Int)
			if !ok {
				panic("invalid uint256 value")
			}
			packed = append(packed, padTo32Bytes(val.Bytes())...)

		case "uint64":
			val, ok := values[i].(uint64)
			if !ok {
				panic("invalid uint64 value")
			}
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, val)
			packed = append(packed, padTo32Bytes(b)...)

		case "address":
			val, ok := values[i].([20]byte)
			if !ok {
				panic("invalid address value")
			}
			packed = append(packed, padTo32Bytes(val[:])...)

		default:
			panic("unsupported type: " + typ)
		}
	}

	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(packed)
	return hasher.Sum(nil)
}

func padTo32Bytes(data []byte) []byte {
	if len(data) > 32 {
		panic("data too long for 32 bytes")
	}
	padded := make([]byte, 32)
	copy(padded[32-len(data):], data)
	return padded
}

func calculateHash(data []interface{}, timestamp int64) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		panic("invalid message to calc hash")
	}
	timestampBig := big.NewInt(timestamp)
	hash := SolidityKeccak256([]string{"string", "uint256"}, []interface{}{string(jsonData), timestampBig})
	log.Printf("Data: %s, Ts: %d, Hash: %x", jsonData, timestampBig, hash)
	return fmt.Sprintf("%x", hash)
}

func FloatToWei(price float64) *big.Int {
	priceBig := new(big.Float).SetFloat64(price)
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	wei := new(big.Float).Mul(priceBig, multiplier)
	result := new(big.Int)
	wei.Int(result)
	return result
}

func (b *StockQuoteMessageBuilder) BuildMessage(price float64) (*SignRequest, error) {
	priceScaled := FloatToWei(price)
	timestamp := time.Now().Unix()

	fieldValues := map[string]interface{}{
		"ticker":            b.Ticker,
		"price":             priceScaled.String(),
		"destination_chain": b.DestinationChain,
		"timestamp":         timestamp,
	}

	dataStructure := make([]string, len(b.Structure.Fields))
	dataStructureMeta := make([]string, len(b.Structure.Fields))
	data := make([]interface{}, len(b.Structure.Fields))

	for i, f := range b.Structure.Fields {
		dataStructure[i] = f.SolidityType
		dataStructureMeta[i] = f.Name
		data[i] = fieldValues[f.Name]
	}

	hash := calculateHash(data, timestamp)

	var dataStructureId int
	if id, err := strconv.Atoi(b.StructureID); err == nil {
		dataStructureId = id
	} else {
		dataStructureId = 0
	}

	return &SignRequest{
		Type:              MsgTypeSignRequest,
		Hash:              hash,
		Data:              data,
		DataStructure:     dataStructure,
		DataStructureMeta: dataStructureMeta,
		DataStructureId:   dataStructureId,
		Timestamp:         timestamp,
	}, nil
}

type MessageFactory struct {
	Ticker      string
	Builders    map[string]func(string, string, DataStructure, int) MessageBuilder
	Structures  map[string]DataStructure
	StructureID string
}

func NewMessageFactory(structureID, ticker string, structures map[string]DataStructure) *MessageFactory {
	return &MessageFactory{
		Ticker:      ticker,
		StructureID: structureID,
		Builders: map[string]func(string, string, DataStructure, int) MessageBuilder{
			"stock_quote": func(ticker, structureID string, structure DataStructure, destChain int) MessageBuilder {
				return &StockQuoteMessageBuilder{
					Ticker:           ticker,
					StructureID:      structureID,
					Structure:        structure,
					DestinationChain: destChain,
				}
			},
		},
		Structures: structures,
	}
}

func (f *MessageFactory) GetBuilder() (MessageBuilder, error) {
	if builderFunc, ok := f.Builders[f.StructureID]; ok {
		if structure, ok := f.Structures[f.StructureID]; ok {
			return builderFunc(f.Ticker, f.StructureID, structure, 1), nil
		}
	}
	return nil, fmt.Errorf("unknown structure_id: %s", f.StructureID)
}

type PriceSource interface {
	FetchPrice(ctx context.Context) (float64, error)
}

func loadDataStructures(filePath string) (map[string]DataStructure, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read data structures file: %v", err)
	}

	var structures map[string]DataStructure
	if err := json.Unmarshal(data, &structures); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data structures: %v", err)
	}

	return structures, nil
}

type PriceAggregator struct {
	Sources []PriceSource
	Timeout time.Duration
}

func (a *PriceAggregator) GetAveragePrice(ctx context.Context) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()

	var total float64
	var count int
	errChan := make(chan error, len(a.Sources))
	resultChan := make(chan float64, len(a.Sources))

	// Fetch prices concurrently
	for _, source := range a.Sources {
		go func(s PriceSource) {
			price, err := s.FetchPrice(ctx)
			if err != nil {
				errChan <- err
				return
			}
			resultChan <- price
		}(source)
	}

	// Collect results
	for i := 0; i < len(a.Sources); i++ {
		select {
		case err := <-errChan:
			log.Printf("Price source error: %v", err)
		case price := <-resultChan:
			total += price
			count++
		case <-ctx.Done():
			return 0, fmt.Errorf("price aggregation timed out")
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid prices received from any source")
	}

	return total / float64(count), nil
}

type Worker struct {
	Aggregator     *PriceAggregator
	PubSub         *PubSubService
	MessageFactory *MessageFactory
	Ticker         string
	StructureID    string
	SleepDelay     time.Duration
	Shutdown       chan struct{}
}

func (w *Worker) Run(ctx context.Context) error {
	builder, err := w.MessageFactory.GetBuilder()
	if err != nil {
		return fmt.Errorf("failed to get message builder: %w", err)
	}

	ticker := time.NewTicker(w.SleepDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.Shutdown:
			return nil
		case <-ticker.C:
			avgPrice, err := w.Aggregator.GetAveragePrice(ctx)
			if err != nil {
				log.Printf("Error getting average price: %v", err)
				continue
			}

			signRequest, err := builder.BuildMessage(avgPrice)
			if err != nil {
				log.Printf("Error building SignRequest: %v", err)
				continue
			}

			if err := w.PubSub.PublishSignRequest(ctx, signRequest); err != nil {
				log.Printf("Error publishing SignRequest: %v", err)
			}
		}
	}
}

type PubSubService struct {
	topic          *pubsub.Topic
	db             Database
	publishTimeout time.Duration
	maxRetries     int
	retryDelay     time.Duration
}

func (s *PubSubService) PublishSignRequest(ctx context.Context, sr *SignRequest) error {
	if err := s.db.StoreData(sr.Hash, sr.Data, sr.DataStructure, sr.DataStructureMeta, sr.Timestamp, sr.DataStructureId); err != nil {
		return fmt.Errorf("failed to store data: %w", err)
	}

	payloadBytes, err := json.Marshal(sr)
	if err != nil {
		return fmt.Errorf("failed to marshal SignRequest: %w", err)
	}

	var lastErr error
	for i := 0; i < s.maxRetries; i++ {
		pubCtx, cancel := context.WithTimeout(ctx, s.publishTimeout)
		err := s.topic.Publish(pubCtx, payloadBytes)
		cancel()

		if err == nil {
			log.Printf("Published SignRequest successfully")
			return nil
		}

		lastErr = err
		log.Printf("Publish attempt %d/%d failed: %v", i+1, s.maxRetries, err)
		time.Sleep(s.retryDelay)
	}

	return fmt.Errorf("failed to publish after %d attempts: %w", s.maxRetries, lastErr)
}
