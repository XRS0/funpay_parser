package main

import (
	"context"
	"log"
	"net"
	"os"

	"funpay-parser/internal/config"
	"funpay-parser/internal/llm"
	"funpay-parser/internal/rpc"
	"google.golang.org/grpc"
)

type server struct{ client *llm.Client }

func (s server) ClassifyMany(ctx context.Context, req *rpc.ClassifyManyRequest) (*rpc.ClassifyManyResponse, error) {
	progress := func(message string) { log.Println(message) }
	return &rpc.ClassifyManyResponse{Listings: s.client.ClassifyMany(ctx, req.Listings, req.Workers, progress)}, nil
}

func main() {
	cfg := config.Load()
	addr := os.Getenv("LLM_GRPC_ADDR")
	if addr == "" {
		addr = ":9091"
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	gs := grpc.NewServer()
	rpc.RegisterLLMService(gs, server{client: llm.New(cfg)})
	log.Println("llm-service gRPC listening", addr)
	log.Fatal(gs.Serve(lis))
}
