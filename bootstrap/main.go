package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

func getOrCreatePrivKey() (crypto.PrivKey, error) {
	pk_str := os.Getenv("PRIVATE_KEY")
	if pk_str == "" {
		priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
		if err != nil {
			return nil, err
		}

		return priv, nil
	}
	pk, err := hex.DecodeString(pk_str)
	if err != nil {
		log.Println("Error decode PK")
	}
	return crypto.UnmarshalSecp256k1PrivateKey([]byte(pk))
}

func parseTrustedAddrsFromEnv() ([]string, error) {
	trustedAddrsStr := os.Getenv("TRUSTED_ADDRESSES")
	if trustedAddrsStr == "" {
		return nil, fmt.Errorf("TRUSTED_ADDRESSES environment variable not set")
	}

	addresses := strings.Split(trustedAddrsStr, ",")
	var result []string

	for _, addr := range addresses {
		trimmed := strings.TrimSpace(addr)
		if !common.IsHexAddress(trimmed) {
			return nil, fmt.Errorf("invalid Ethereum address: %s", trimmed)
		}
		result = append(result, trimmed)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid addresses found in TRUSTED_ADDRESSES")
	}

	return result, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found")
	}

	trustedAddrs, err := parseTrustedAddrsFromEnv()
	if err != nil {
		log.Fatalf("Failed to parse trusted addresses: %v", err)
	}

	privKey, err := getOrCreatePrivKey()
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	topicName := os.Getenv("TOPIC")
	if topicName == "" {
		log.Fatal("TOPIC environment variable not set")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/leveldb"
	}

	log.Printf("Opening database at %s", dbPath)
	db, err := NewLevelDBDatabase(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		log.Println("Cleaning up resources...")
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
		cancel()
	}

	operator, err := NewOperatorNode(ctx, cancel, privKey, db, topicName, trustedAddrs)
	if err != nil {
		cleanup()
		log.Fatalf("Failed to create operator node: %v", err)
	}

	rpcPort := os.Getenv("RPC_PORT")
	if rpcPort == "" {
		rpcPort = "8080"
	}
	rpcServer := NewRPCServer(operator, rpcPort)

	// Start data collector
	interval := dataCollectionInterval
	if intervalEnv := os.Getenv("DATA_COLLECTION_INTERVAL"); intervalEnv != "" {
		if i, err := strconv.Atoi(intervalEnv); err == nil {
			interval = i
		}
	}

	tickers := []string{"SBER"}
	if tickersEnv := os.Getenv("TICKERS"); tickersEnv != "" {
		tickers = strings.Split(tickersEnv, ",")
	}

	structuresFilePath := "config/data_structures.json"
	if structuresPathEnv := os.Getenv("DATA_STRUCTURES_PATH"); structuresPathEnv != "" {
		structuresFilePath = structuresPathEnv
	}

	var workers []*Worker

	structures, err := loadDataStructures(structuresFilePath)
	if err != nil {
		log.Printf("Warning: Failed to load data structures: %v", err)
	} else {
		for _, ticker := range tickers {
			structureID := "stock_quote"

			sources := CreatePriceSources(ticker)

			aggregator := &PriceAggregator{
				Sources: sources,
				Timeout: 15 * time.Second,
			}

			factory := NewMessageFactory(structureID, ticker, structures)

			pubSubService := &PubSubService{
				topic:          operator.topic,
				db:             db,
				publishTimeout: 10 * time.Second,
				maxRetries:     3,
				retryDelay:     2 * time.Second,
			}

			worker := &Worker{
				Aggregator:     aggregator,
				PubSub:         pubSubService,
				MessageFactory: factory,
				Ticker:         ticker,
				StructureID:    structureID,
				SleepDelay:     time.Duration(interval) * time.Second,
				Shutdown:       make(chan struct{}),
			}

			workers = append(workers, worker)

			go func(w *Worker, t string) {
				log.Printf("Starting data source worker for %s", t)
				if err := w.Run(ctx); err != nil {
					log.Printf("Error running data source worker for %s: %v", t, err)
				}
			}(worker, ticker)
		}

		log.Println("✅ Data source workers started")
	}

	go rpcServer.Start()
	log.Println("✅ RPC server started")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	for _, worker := range workers {
		log.Printf("Stopping worker for %s", worker.Ticker)
		close(worker.Shutdown)
	}

	if err := rpcServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down RPC server: %v", err)
	}

	operator.gracefulShutdown()
}
