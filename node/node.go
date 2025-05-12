package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	MsgTypeSignRequest  = "sign_request"
	MsgTypeSignResponse = "sign_response"
)

type SignRequest struct {
	Type string `json:"type"`
	Hash string `json:"hash"`
}

type SignResponse struct {
	Type      string `json:"type"`
	Hash      string `json:"hash"`
	Signature string `json:"signature"`
	PeerID    string `json:"peer_id"`
}

type Node struct {
	ctx       context.Context
	host      host.Host
	topic     *pubsub.Topic
	sub       *pubsub.Subscription
	signer    Signer
	bootstrap string
	wg        sync.WaitGroup
}

type Signer interface {
	Sign(message []byte) (string, error)
	Address() string
}

func NewNode(ctx context.Context, privKey crypto.PrivKey, signer Signer, topicName, bootstrapAddr string) (*Node, error) {
	h, err := libp2p.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	log.Println("✅ Node started.")

	ps, err := pubsub.NewGossipSub(ctx, h)
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

	node := &Node{
		ctx:       ctx,
		host:      h,
		topic:     topic,
		sub:       sub,
		signer:    signer,
		bootstrap: bootstrapAddr,
	}

	node.setupNetworkNotifiers()
	node.connectToBootstrap()
	go node.listen()
	go node.connectionMonitor()
	return node, nil
}

func (n *Node) setupNetworkNotifiers() {
	n.host.Network().Notify(&network.NotifyBundle{
		DisconnectedF: func(net network.Network, conn network.Conn) {
			log.Printf("❌ Disconnected from peer: %s", conn.RemotePeer())
		},
	})
}

func (n *Node) connectionMonitor() {
	ticker := time.NewTicker(connectionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			if n.bootstrap != "" && len(n.host.Network().Peers()) == 0 {
				log.Println("⚠️ No peers connected, attempting to reconnect to bootstrap...")
				n.connectToBootstrap()
			}
		}
	}
}

func (n *Node) connectToBootstrap() {
	if n.bootstrap == "" {
		return
	}

	maddr, err := multiaddr.NewMultiaddr(n.bootstrap)
	if err != nil {
		log.Printf("Error parsing bootstrap address: %v", err)
		return
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		log.Printf("Error getting bootstrap peer info: %v", err)
		return
	}

	for {
		ctx, cancel := context.WithTimeout(n.ctx, reconnectTimeout)
		err := n.host.Connect(ctx, *peerInfo)
		cancel()

		if err == nil {
			log.Println("✅ Connected to bootstrap node")
			return
		}

		log.Printf("Reconnect attempt failed: %v", err)
		time.Sleep(reconnectTimeout)
	}

}

func (n *Node) resubscribe() error {
	var err error
	n.sub.Cancel()

	for i := 0; i < maxReconnectAttempts; i++ {
		n.sub, err = n.topic.Subscribe()
		if err == nil {
			return nil
		}
		log.Printf("Resubscribe attempt %d/%d failed: %v", i+1, maxReconnectAttempts, err)
		time.Sleep(reconnectTimeout)
	}

	return fmt.Errorf("failed to resubscribe after %d attempts: %w", maxReconnectAttempts, err)
}

func (n *Node) listen() {
	defer n.wg.Done()
	n.wg.Add(1)

	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			ctx, cancel := context.WithTimeout(n.ctx, subscriptionReadTimeout)
			msg, err := n.sub.Next(ctx)
			cancel()

			if err != nil {
				if n.ctx.Err() == nil {
					log.Printf("Error reading from subscription: %v", err)
					if err := n.resubscribe(); err != nil {
						log.Printf("Failed to resubscribe: %v", err)
					}
				}
				continue
			}

			n.HandleMessage(msg.Data)
		}
	}
}

func (n *Node) HandleMessage(data []byte) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	switch msg.Type {
	case MsgTypeSignRequest:
		var req SignRequest
		if err := json.Unmarshal(data, &req); err != nil {
			log.Printf("Error unmarshaling sign request: %v", err)
			return
		}
		log.Printf("Processing sign request for: %s", req.Hash)
		n.handleSignRequest(&req)
	default:
	}
}

func (n *Node) handleSignRequest(req *SignRequest) {
	// Decode the hex string
	hash, err := hex.DecodeString(req.Hash)
	if err != nil {
		panic(err)
	}
	message := accounts.TextHash(hash)

	signature, err := n.signer.Sign(message)
	if err != nil {
		log.Printf("Error signing data: %v", err)
		return
	}

	resp := SignResponse{
		Type:      MsgTypeSignResponse,
		Hash:      req.Hash,
		Signature: signature,
		PeerID:    n.signer.Address(),
	}

	msg, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling sign response: %v", err)
		return
	}

	if err := n.topic.Publish(n.ctx, msg); err != nil {
		log.Printf("Error publishing sign response: %v", err)
	}
}
