package main

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	cryptoeth "github.com/ethereum/go-ethereum/crypto"
	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	reconnectTimeout         = 5 * time.Second
	maxReconnectAttempts     = 10
	publishTimeout           = 10 * time.Second
	subscriptionReadTimeout  = 60 * time.Second
	peerDiscoveryInterval    = 60 * time.Second
	peerGarbageCollectorTime = 5 * time.Minute
	dataCollectionInterval   = 3
)

const (
	MsgTypeSignRequest  = "sign_request"
	MsgTypeSignResponse = "sign_response"
)

type SignRequest struct {
	Type              string        `json:"type"`
	Hash              string        `json:"hash"`
	Data              []interface{} `json:"data"`
	DataStructure     []string      `json:"data_structure"`
	DataStructureMeta []string      `json:"data_structure_meta"`
	DataStructureId   int           `json:"data_structure_id"`
	Timestamp         int64         `json:"timestamp"`
}

type SignResponse struct {
	Type      string `json:"type"`
	Hash      string `json:"hash"`
	Signature string `json:"signature"`
	PeerID    string `json:"peer_id"`
}

type PendingRequest struct {
	timestamp time.Time
	signers   map[string]bool
	data      SignRequest
}

type OperatorNode struct {
	ctx             context.Context
	cancel          context.CancelFunc
	host            host.Host
	topic           *pubsub.Topic
	sub             *pubsub.Subscription
	db              Database
	pending         map[string]*PendingRequest
	pendingExpiry   time.Duration
	pendingMux      sync.RWMutex
	trustedAddrs    []string
	knownPeers      map[peer.ID]time.Time
	knownPeersMux   sync.RWMutex
	lastMessageTime time.Time
}

func NewOperatorNode(ctx context.Context, cancel context.CancelFunc, privKey crypto.PrivKey, db Database, topicName string, trustedAddrs []string) (*OperatorNode, error) {
	host, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/4001"),
		libp2p.Identity(privKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	log.Println("✅ Bootstrap node started.")

	for _, addr := range host.Addrs() {
		fullAddr := fmt.Sprintf("%s/p2p/%s", addr, host.ID().String())
		log.Println("🛰️ Listening on:", fullAddr)
	}

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	operator := &OperatorNode{
		ctx:           ctx,
		cancel:        cancel,
		host:          host,
		topic:         topic,
		sub:           sub,
		db:            db,
		pending:       make(map[string]*PendingRequest),
		trustedAddrs:  trustedAddrs,
		knownPeers:    make(map[peer.ID]time.Time),
		pendingExpiry: 5 * time.Minute,
	}

	// Setup network notifiers
	host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(net network.Network, conn network.Conn) {
			peerID := conn.RemotePeer()
			operator.knownPeersMux.Lock()
			operator.knownPeers[peerID] = time.Now()
			operator.knownPeersMux.Unlock()
			log.Printf("🔗 New peer connected: %s", peerID)
		},
		DisconnectedF: func(net network.Network, conn network.Conn) {
			peerID := conn.RemotePeer()
			log.Printf("❌ Peer disconnected: %s", peerID)
		},
	})

	go operator.listen()
	go operator.retryPendingRequests()
	go operator.peerDiscovery()
	go operator.peerGarbageCollector()
	go operator.healthMonitor()

	return operator, nil
}

func (o *OperatorNode) peerDiscovery() {
	ticker := time.NewTicker(peerDiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.knownPeersMux.RLock()
			peerCount := len(o.knownPeers)
			o.knownPeersMux.RUnlock()

			log.Printf("🌐 Known peers: %d", peerCount)

			if peerCount == 0 {
				// Attempt to find peers through DHT or other discovery mechanisms
				log.Println("⚠️ No peers connected, attempting active peer discovery...")

				peersToTry := o.host.Peerstore().Peers()
				if len(peersToTry) > 0 {
					log.Printf("Attempting to reconnect to %d known peers in peerstore", len(peersToTry))
					for _, peerID := range peersToTry {
						if peerID == o.host.ID() {
							continue
						}

						if o.host.Network().Connectedness(peerID) == network.Connected {
							continue
						}

						addrs := o.host.Peerstore().Addrs(peerID)
						if len(addrs) == 0 {
							continue
						}

						ctx, cancel := context.WithTimeout(o.ctx, 5*time.Second)
						err := o.host.Connect(ctx, peer.AddrInfo{
							ID:    peerID,
							Addrs: addrs,
						})
						cancel()

						if err != nil {
							log.Printf("Failed to reconnect to peer %s: %v", peerID, err)
						} else {
							log.Printf("Successfully reconnected to peer %s", peerID)
						}
					}
				}
			}
		}
	}
}

func (o *OperatorNode) peerGarbageCollector() {
	ticker := time.NewTicker(peerGarbageCollectorTime)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			o.knownPeersMux.Lock()
			for p, lastSeen := range o.knownPeers {
				if now.Sub(lastSeen) > peerGarbageCollectorTime {
					delete(o.knownPeers, p)
				}
			}
			o.knownPeersMux.Unlock()
		}
	}
}

func (o *OperatorNode) threshold() int {
	return len(o.trustedAddrs)/2 + 1
}

func (o *OperatorNode) listen() {
	for {
		select {
		case <-o.ctx.Done():
			return
		default:
			ctx, cancel := context.WithTimeout(o.ctx, subscriptionReadTimeout)
			msg, err := o.sub.Next(ctx)
			cancel()

			if err != nil {
				if o.ctx.Err() == nil {
					if err == context.DeadlineExceeded {
						log.Printf("Чтение из подписки превысило таймаут (%v). Переподключение...", subscriptionReadTimeout)
					} else {
						log.Printf("Ошибка при чтении из подписки: %v. Переподключение...", err)
					}

					if err := o.resubscribe(); err != nil {
						log.Printf("Критическая ошибка при переподключении: %v", err)
						time.Sleep(5 * time.Second)
					}
					continue
				}
				return // Exit if context is done
			}

			o.HandleMessage(msg.Data)
		}
	}
}

func (o *OperatorNode) resubscribe() error {
	if o.sub != nil {
		o.sub.Cancel()
	}

	var err error
	for i := 0; i < maxReconnectAttempts; i++ {
		o.sub, err = o.topic.Subscribe()
		if err == nil {
			log.Println("✅ Успешно переподключились к топику")
			return nil
		}

		log.Printf("Попытка переподключения %d/%d не удалась: %v",
			i+1, maxReconnectAttempts, err)

		sleepTime := reconnectTimeout * time.Duration(i+1)
		if sleepTime > 30*time.Second {
			sleepTime = 30 * time.Second
		}

		select {
		case <-o.ctx.Done():
			return fmt.Errorf("Контекст отменен при переподключении: %w", o.ctx.Err())
		case <-time.After(sleepTime):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("Не удалось переподключиться после %d попыток: %w", maxReconnectAttempts, err)
}

func (o *OperatorNode) healthMonitor() {
	healthCheckTicker := time.NewTicker(30 * time.Second)
	defer healthCheckTicker.Stop()

	consecutiveTimeouts := 0
	maxConsecutiveTimeouts := 3

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-healthCheckTicker.C:
			o.knownPeersMux.RLock()
			hasRecentMessage := !o.lastMessageTime.IsZero() && time.Since(o.lastMessageTime) <= 5*time.Minute
			o.knownPeersMux.RUnlock()

			if !hasRecentMessage {
				log.Printf("⚠️ No messages received in 5 minutes, health check triggered")

				o.knownPeersMux.RLock()
				peerCount := len(o.knownPeers)
				o.knownPeersMux.RUnlock()

				if peerCount == 0 {
					log.Println("🔄 No peers connected, forcing peer discovery")
					peersToTry := o.host.Peerstore().Peers()
					for _, peerID := range peersToTry {
						if peerID == o.host.ID() {
							continue
						}

						addrs := o.host.Peerstore().Addrs(peerID)
						if len(addrs) == 0 {
							continue
						}

						ctx, cancel := context.WithTimeout(o.ctx, 5*time.Second)
						err := o.host.Connect(ctx, peer.AddrInfo{
							ID:    peerID,
							Addrs: addrs,
						})
						cancel()

						if err == nil {
							log.Printf("✅ Successfully reconnected to peer %s", peerID)
						}
					}

					if consecutiveTimeouts >= maxConsecutiveTimeouts {
						log.Println("🔄 Multiple timeouts detected, attempting to reset subscription")
						if err := o.resubscribe(); err != nil {
							log.Printf("❌ Failed to resubscribe: %v", err)
						} else {
							consecutiveTimeouts = 0
						}
					} else {
						consecutiveTimeouts++
					}
				} else {
					consecutiveTimeouts = 0
				}
			} else {
				consecutiveTimeouts = 0
			}
		}
	}
}

func (o *OperatorNode) retryPendingRequests() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	tickerExpired := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.pendingMux.RLock()
			pendingHashes := make([]string, 0, len(o.pending))
			for hash := range o.pending {
				pendingHashes = append(pendingHashes, hash)
			}
			o.pendingMux.RUnlock()

			for _, hash := range pendingHashes {
				o.BroadcastSignRequest(hash)
			}
		case <-tickerExpired.C:
			o.cleanupExpiredRequests()
		}

	}
}

func (o *OperatorNode) cleanupExpiredRequests() {
	o.pendingMux.Lock()
	defer o.pendingMux.Unlock()

	now := time.Now()
	for hash, req := range o.pending {
		if now.Sub(req.timestamp) > o.pendingExpiry {
			delete(o.pending, hash)
			log.Printf("Expired pending request: %s", hash)
		}
	}
}

func (o *OperatorNode) gracefulShutdown() {
	log.Println("Shutting down...")

	o.cancel()

	if o.sub != nil {
		o.sub.Cancel()
	}

	if o.host != nil {
		if err := o.host.Close(); err != nil {
			log.Printf("Error closing host: %v", err)
		}
	}

	if err := o.db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
}

func (o *OperatorNode) BroadcastSignRequest(hash string) error {
	req := SignRequest{
		Type: MsgTypeSignRequest,
		Hash: hash,
	}

	msg, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(o.ctx, publishTimeout)
	defer cancel()

	return o.topic.Publish(ctx, msg)
}

func verifySignature(message []byte, signatureHex string) (common.Address, error) {
	sigBytes, err := hexutil.Decode(signatureHex)
	if err != nil {
		return common.Address{}, fmt.Errorf("invalid signature hex: %v", err)
	}

	if len(sigBytes) != 65 {
		return common.Address{}, fmt.Errorf("invalid signature length, expected 65 got %d", len(sigBytes))
	}

	pubKey, err := cryptoeth.SigToPub(message, sigBytes)
	if err != nil {
		return common.Address{}, fmt.Errorf("signature recovery failed: %v", err)
	}

	recoveredAddr := cryptoeth.PubkeyToAddress(*pubKey)

	return recoveredAddr, nil
}

func (o *OperatorNode) handleSignResponse(resp *SignResponse) {
	log.Printf("Received signature response for hash: %s from %s", resp.Hash, resp.PeerID)

	hash, err := hex.DecodeString(resp.Hash)
	if err != nil {
		panic(err)
	}

	message := accounts.TextHash(hash)

	signerAddress, err := verifySignature(message, resp.Signature)
	if err != nil {
		log.Printf("Signature verification failed: %v", err)
		return
	}

	isTrusted := false
	for _, addr := range o.trustedAddrs {
		if strings.EqualFold(signerAddress.Hex(), addr) {
			isTrusted = true
			break
		}
	}

	if !isTrusted {
		log.Printf("Untrusted signer: %s", signerAddress.Hex())
		return
	}

	o.pendingMux.Lock()
	defer o.pendingMux.Unlock()

	req, exists := o.pending[resp.Hash]
	if !exists {
		return
	}

	if err := o.db.StoreSignature(resp.Hash, signerAddress.Hex(), resp.Signature); err != nil {
		log.Printf("Error storing signature: %v", err)
		return
	}

	req.signers[signerAddress.Hex()] = true
	log.Printf("Stored signature for %s from %s (total: %d)", resp.Hash, signerAddress.Hex(), len(req.signers))

	if len(req.signers) >= o.threshold() {
		log.Printf("✅ Reached threshold %d of %d for %s", len(req.signers), len(o.trustedAddrs), resp.Hash)
		if len(req.signers) == len(o.trustedAddrs) {
			delete(o.pending, resp.Hash)
		}
	}
}

func (o *OperatorNode) HandleMessage(data []byte) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	o.knownPeersMux.Lock()
	o.lastMessageTime = time.Now()
	o.knownPeersMux.Unlock()

	switch msg.Type {
	case MsgTypeSignRequest:
		var req SignRequest
		if err := json.Unmarshal(data, &req); err != nil {
			log.Printf("Error unmarshaling sign request: %v", err)
			return
		}
		o.handleSignRequest(&req)
	case MsgTypeSignResponse:
		var resp SignResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			log.Printf("Error unmarshaling sign response: %v", err)
			return
		}
		o.handleSignResponse(&resp)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

func (o *OperatorNode) handleSignRequest(req *SignRequest) {
	o.pendingMux.Lock()
	if _, exists := o.pending[req.Hash]; !exists {
		o.pending[req.Hash] = &PendingRequest{
			timestamp: time.Now(),
			signers:   make(map[string]bool),
			data:      *req,
		}
	}
	o.pendingMux.Unlock()
}
