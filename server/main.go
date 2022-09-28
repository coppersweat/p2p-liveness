package main

import (
	"context"
	"crypto/rand"
	"log"
	"net"
	"time"

	pb "github.com/coppersweat/p2p-liveness/proto"
	"github.com/libp2p/go-libp2p"
	peerstore "github.com/libp2p/go-libp2p-core/peer"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
	"google.golang.org/grpc"
)

const (
	port = ":50052"
)

type server struct {
	multiaddresses []*pb.Request
}

func makeNode() (host.Host, error) {
	// Generate a key pair for the node
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, err
	}

	// Create a new node
	node, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		libp2p.Identity(priv),
		libp2p.Ping(false),
	)
	if err != nil {
		return nil, err
	}

	// Configure our own ping protocol
	pingService := &ping.PingService{Host: node}
	node.SetStreamHandler(ping.ID, pingService.PingHandler)

	return node, nil
}

func (s *server) GetNodesLiveness(multiaddresses *pb.Request, stream pb.Liveness_GetNodesLivenessServer) error {
	// Get the requested multiaddresses
	s.multiaddresses = append(s.multiaddresses, multiaddresses)

	// Create host in the p2p network
	h, err := makeNode()
	if err != nil {
		return err
	}
	defer h.Close()

	// Connect the host to the requested nodes to be pinged
	for _, multiaddrReq := range s.multiaddresses {
		for _, multiaddr := range multiaddrReq.Multiaddresses {
			addr, err := ma.NewMultiaddr(multiaddr)
			if err != nil {
				return err
			}
			p, err := peerstore.AddrInfoFromP2pAddr(addr)
			if err != nil {
				return err
			}
			if err := h.Connect(context.Background(), *p); err != nil {
				return err
			}
		}
	}

	// Ping the requested nodes in the network and stream output the results
	for {
		for _, multiaddrReq := range s.multiaddresses {
			for _, multiaddr := range multiaddrReq.Multiaddresses {
				addr, err := ma.NewMultiaddr(multiaddr)
				if err != nil {
					return err
				}
				p, err := peerstore.AddrInfoFromP2pAddr(addr)
				if err != nil {
					return err
				}
				pingService := &ping.PingService{Host: h}
				ch := pingService.Ping(context.Background(), p.ID)
				var mess string
				pingRes := <-ch
				if err != nil {
					mess = "pinged " + multiaddr + " errored: " + err.Error()
				} else {
					mess = "pinged " + multiaddr + " in " + pingRes.RTT.String()
				}
				resp := &pb.Response{
					Pingmessage: mess,
				}
				if err := stream.Send(resp); err != nil {
					return err
				}
			}
		}
		// Wait until next round of pings
		time.Sleep(2 * time.Second)
	}
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	// Creates a new gRPC server
	s := grpc.NewServer()
	pb.RegisterLivenessServer(s, &server{})
	s.Serve(lis)
}
