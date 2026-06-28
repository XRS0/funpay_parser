package rpc

import (
	"context"

	"funpay-parser/internal/models"
	"funpay-parser/internal/runner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ParserClient struct{ cc *grpc.ClientConn }

func DialParser(addr string) (*ParserClient, error) {
	cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})))
	if err != nil {
		return nil, err
	}
	return &ParserClient{cc: cc}, nil
}
func (c *ParserClient) Close() error { return c.cc.Close() }
func (c *ParserClient) Run(ctx context.Context, opt runner.Options, progress func(string)) (runner.Result, error) {
	var out RunParserResponse
	err := c.cc.Invoke(ctx, "/funpay.ParserService/RunParser", &RunParserRequest{Options: opt}, &out)
	for _, p := range out.Progress {
		if progress != nil {
			progress(p.Message)
		}
	}
	return out.Result, err
}

type LLMClient struct{ cc *grpc.ClientConn }

func DialLLM(addr string) (*LLMClient, error) {
	cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})))
	if err != nil {
		return nil, err
	}
	return &LLMClient{cc: cc}, nil
}
func (c *LLMClient) Close() error { return c.cc.Close() }
func (c *LLMClient) ClassifyMany(ctx context.Context, listings []models.Listing, workers int, progress func(string)) []models.Listing {
	var out ClassifyManyResponse
	err := c.cc.Invoke(ctx, "/funpay.LLMService/ClassifyMany", &ClassifyManyRequest{Listings: listings, Workers: workers}, &out)
	if err != nil {
		for i := range listings {
			listings[i].ClassificationReason = "llm-service error: " + err.Error()
		}
		return listings
	}
	return out.Listings
}
