package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	cryptoeth "github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
	"github.com/libp2p/go-libp2p/core/crypto"
)

const (
	reconnectTimeout        = 5 * time.Second
	maxReconnectAttempts    = 30
	connectionCheckInterval = 10 * time.Second
	subscriptionReadTimeout = 30 * time.Second
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

type MemorySigner struct {
	privKey      crypto.PrivKey
	ecdsaPrivKey ecdsa.PrivateKey
	address      string
}

func NewMemorySigner(privKey crypto.PrivKey) (*MemorySigner, error) {
	raw, err := privKey.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw private key: %w", err)
	}

	ecdsaPrivKey, err := cryptoeth.ToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to ECDSA key: %w", err)
	}

	address := cryptoeth.PubkeyToAddress(ecdsaPrivKey.PublicKey)
	log.Println("Signer", address)

	return &MemorySigner{
		privKey:      privKey,
		ecdsaPrivKey: *ecdsaPrivKey,
		address:      address.Hex(),
	}, nil
}

func (s *MemorySigner) Sign(message []byte) (string, error) {
	signature, err := cryptoeth.Sign(message, &s.ecdsaPrivKey)
	if err != nil {
		return "", err
	}

	return hexutil.Encode(signature), nil
}

func (s *MemorySigner) Address() string {
	return s.address
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found")
	}

	operatorAddr := os.Getenv("BOOTSTRAP_NODE")
	topic := os.Getenv("TOPIC")

	privKey, err := getOrCreatePrivKey()
	if err != nil {
		log.Fatal(err)
	}
	signer, err := NewMemorySigner(privKey)
	if err != nil {
		log.Fatal(err)
	}

	node, err := NewNode(ctx, privKey, signer, topic, operatorAddr)
	if err != nil {
		log.Fatalf("Failed to create regular node: %v", err)
	}

	<-ctx.Done()
	node.wg.Wait()
}
