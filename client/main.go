package main

import (
	"crypto/rand"
	"io"
	"log"

	pb "github.com/coppersweat/p2p-liveness/proto"
	"github.com/libp2p/go-libp2p"
	peerstore "github.com/libp2p/go-libp2p-core/peer"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	address = "localhost:50052"
)

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

func getAddress(h host.Host) (string, error) {
	peerInfo := peerstore.AddrInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
	addrs, err := peerstore.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		return "", err
	}
	return addrs[0].String(), nil
}

func createNetwork() ([]string, error) {
	// To create 4 random nodes; testing purposes only
	const totalnodes = 4
	var nodes []string

	for i := 0; i < totalnodes; i++ {
		// Create a new node
		n, err := makeNode()
		if err != nil {
			return nil, err
		}

		// Get the address of the new node
		addr, err := getAddress(n)
		if err != nil {
			return nil, err
		}

		// Add the multiaddress of the node to the resulting array of multiaddr
		nodes = append(nodes, addr)
	}

	return nodes, nil
}

// getNodesLiveness calls the RPC method GetNodesLiveness of LivenessServer
func getNodesLiveness(client pb.LivenessClient, multiaddresses *pb.Request) {
	// calling the streaming API
	stream, err := client.GetNodesLiveness(context.Background(), multiaddresses)
	if err != nil {
		log.Fatalf("Error on getting the liveness of nodes: %v", err)
	}
	for {
		// Receiving the stream of data
		pingmessage, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.GetNodesLiveness(_) = _, %v", client, err)
		}
		log.Printf("Response: %v", pingmessage)
	}
}

func main() {
	// Set up a connection to the gRPC server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	// Creates a new LivenessClient
	client := pb.NewLivenessClient(conn)

	// Create nodes in the network; for testing purposes only (they're supposed to be there already!)
	nodes, err := createNetwork()
	if err != nil {
		log.Fatalf("fail to create network: %v", err)
	}
	multiaddresses := &pb.Request{Multiaddresses: nodes}

	// Ping the nodes to check their liveness
	getNodesLiveness(client, multiaddresses)
}
