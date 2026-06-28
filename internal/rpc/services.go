package rpc

import (
	"context"

	"google.golang.org/grpc"
)

type ParserService interface {
	RunParser(context.Context, *RunParserRequest) (*RunParserResponse, error)
}

type LLMService interface {
	ClassifyMany(context.Context, *ClassifyManyRequest) (*ClassifyManyResponse, error)
}

func RegisterParserService(s *grpc.Server, impl ParserService) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "funpay.ParserService",
		HandlerType: (*ParserService)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "RunParser",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(RunParserRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(ParserService).RunParser(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funpay.ParserService/RunParser"}
				handler := func(ctx context.Context, req any) (any, error) {
					return srv.(ParserService).RunParser(ctx, req.(*RunParserRequest))
				}
				return interceptor(ctx, in, info, handler)
			},
		}},
	}, impl)
}

func RegisterLLMService(s *grpc.Server, impl LLMService) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "funpay.LLMService",
		HandlerType: (*LLMService)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "ClassifyMany",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(ClassifyManyRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(LLMService).ClassifyMany(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/funpay.LLMService/ClassifyMany"}
				handler := func(ctx context.Context, req any) (any, error) {
					return srv.(LLMService).ClassifyMany(ctx, req.(*ClassifyManyRequest))
				}
				return interceptor(ctx, in, info, handler)
			},
		}},
	}, impl)
}
