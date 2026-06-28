package main

import (
	"context"
	"log"
	"net"
	"os"

	"funpay-parser/internal/config"
	"funpay-parser/internal/rpc"
	"funpay-parser/internal/runner"
	"google.golang.org/grpc"
)

type server struct {
	cfg config.Config
	llm runner.Classifier
}

func (s server) RunParser(ctx context.Context, req *rpc.RunParserRequest) (*rpc.RunParserResponse, error) {
	progress := []rpc.ProgressEvent{}
	res, err := runner.RunWithClassifier(ctx, s.cfg, req.Options, func(message string) {
		log.Println(message)
		progress = append(progress, rpc.ProgressEvent{Message: message})
	}, s.llm)
	return &rpc.RunParserResponse{Result: res, Progress: progress}, err
}

func main() {
	cfg := config.Load()
	addr := os.Getenv("PARSER_GRPC_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	var classifier runner.Classifier
	if cfg.LLMServiceAddr != "" {
		client, err := rpc.DialLLM(cfg.LLMServiceAddr)
		if err != nil {
			log.Println("llm-service unavailable, using local llm:", err)
		} else {
			classifier = client
			log.Println("parser-service: using llm-service", cfg.LLMServiceAddr)
		}
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	gs := grpc.NewServer()
	rpc.RegisterParserService(gs, server{cfg: cfg, llm: classifier})
	log.Println("parser-service gRPC listening", addr)
	log.Fatal(gs.Serve(lis))
}
