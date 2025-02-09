// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.9
// source: macro_benchmark.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// BenchmarkClient is the client API for Benchmark service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BenchmarkClient interface {
	StartMacroBenchmark(ctx context.Context, in *StartMacroBenchmarkRequest, opts ...grpc.CallOption) (Benchmark_StartMacroBenchmarkClient, error)
	GetMacroBenchmark(ctx context.Context, in *GetMacroBenchmarkRequest, opts ...grpc.CallOption) (*GetMacroBenchmarkResponse, error)
	ListMacroBenchmarks(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ListMacroBenchmarksResponse, error)
	Status(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*StatusResponse, error)
}

type benchmarkClient struct {
	cc grpc.ClientConnInterface
}

func NewBenchmarkClient(cc grpc.ClientConnInterface) BenchmarkClient {
	return &benchmarkClient{cc}
}

func (c *benchmarkClient) StartMacroBenchmark(ctx context.Context, in *StartMacroBenchmarkRequest, opts ...grpc.CallOption) (Benchmark_StartMacroBenchmarkClient, error) {
	stream, err := c.cc.NewStream(ctx, &Benchmark_ServiceDesc.Streams[0], "/benchmark.Benchmark/StartMacroBenchmark", opts...)
	if err != nil {
		return nil, err
	}
	x := &benchmarkStartMacroBenchmarkClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Benchmark_StartMacroBenchmarkClient interface {
	Recv() (*StartMacroBenchmarkResponse, error)
	grpc.ClientStream
}

type benchmarkStartMacroBenchmarkClient struct {
	grpc.ClientStream
}

func (x *benchmarkStartMacroBenchmarkClient) Recv() (*StartMacroBenchmarkResponse, error) {
	m := new(StartMacroBenchmarkResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *benchmarkClient) GetMacroBenchmark(ctx context.Context, in *GetMacroBenchmarkRequest, opts ...grpc.CallOption) (*GetMacroBenchmarkResponse, error) {
	out := new(GetMacroBenchmarkResponse)
	err := c.cc.Invoke(ctx, "/benchmark.Benchmark/GetMacroBenchmark", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *benchmarkClient) ListMacroBenchmarks(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ListMacroBenchmarksResponse, error) {
	out := new(ListMacroBenchmarksResponse)
	err := c.cc.Invoke(ctx, "/benchmark.Benchmark/ListMacroBenchmarks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *benchmarkClient) Status(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*StatusResponse, error) {
	out := new(StatusResponse)
	err := c.cc.Invoke(ctx, "/benchmark.Benchmark/Status", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BenchmarkServer is the server API for Benchmark service.
// All implementations must embed UnimplementedBenchmarkServer
// for forward compatibility
type BenchmarkServer interface {
	StartMacroBenchmark(*StartMacroBenchmarkRequest, Benchmark_StartMacroBenchmarkServer) error
	GetMacroBenchmark(context.Context, *GetMacroBenchmarkRequest) (*GetMacroBenchmarkResponse, error)
	ListMacroBenchmarks(context.Context, *emptypb.Empty) (*ListMacroBenchmarksResponse, error)
	Status(context.Context, *emptypb.Empty) (*StatusResponse, error)
	mustEmbedUnimplementedBenchmarkServer()
}

// UnimplementedBenchmarkServer must be embedded to have forward compatible implementations.
type UnimplementedBenchmarkServer struct {
}

func (UnimplementedBenchmarkServer) StartMacroBenchmark(*StartMacroBenchmarkRequest, Benchmark_StartMacroBenchmarkServer) error {
	return status.Errorf(codes.Unimplemented, "method StartMacroBenchmark not implemented")
}
func (UnimplementedBenchmarkServer) GetMacroBenchmark(context.Context, *GetMacroBenchmarkRequest) (*GetMacroBenchmarkResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMacroBenchmark not implemented")
}
func (UnimplementedBenchmarkServer) ListMacroBenchmarks(context.Context, *emptypb.Empty) (*ListMacroBenchmarksResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListMacroBenchmarks not implemented")
}
func (UnimplementedBenchmarkServer) Status(context.Context, *emptypb.Empty) (*StatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Status not implemented")
}
func (UnimplementedBenchmarkServer) mustEmbedUnimplementedBenchmarkServer() {}

// UnsafeBenchmarkServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BenchmarkServer will
// result in compilation errors.
type UnsafeBenchmarkServer interface {
	mustEmbedUnimplementedBenchmarkServer()
}

func RegisterBenchmarkServer(s grpc.ServiceRegistrar, srv BenchmarkServer) {
	s.RegisterService(&Benchmark_ServiceDesc, srv)
}

func _Benchmark_StartMacroBenchmark_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StartMacroBenchmarkRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(BenchmarkServer).StartMacroBenchmark(m, &benchmarkStartMacroBenchmarkServer{stream})
}

type Benchmark_StartMacroBenchmarkServer interface {
	Send(*StartMacroBenchmarkResponse) error
	grpc.ServerStream
}

type benchmarkStartMacroBenchmarkServer struct {
	grpc.ServerStream
}

func (x *benchmarkStartMacroBenchmarkServer) Send(m *StartMacroBenchmarkResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _Benchmark_GetMacroBenchmark_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetMacroBenchmarkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BenchmarkServer).GetMacroBenchmark(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/benchmark.Benchmark/GetMacroBenchmark",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BenchmarkServer).GetMacroBenchmark(ctx, req.(*GetMacroBenchmarkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Benchmark_ListMacroBenchmarks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BenchmarkServer).ListMacroBenchmarks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/benchmark.Benchmark/ListMacroBenchmarks",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BenchmarkServer).ListMacroBenchmarks(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Benchmark_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BenchmarkServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/benchmark.Benchmark/Status",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BenchmarkServer).Status(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// Benchmark_ServiceDesc is the grpc.ServiceDesc for Benchmark service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Benchmark_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "benchmark.Benchmark",
	HandlerType: (*BenchmarkServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetMacroBenchmark",
			Handler:    _Benchmark_GetMacroBenchmark_Handler,
		},
		{
			MethodName: "ListMacroBenchmarks",
			Handler:    _Benchmark_ListMacroBenchmarks_Handler,
		},
		{
			MethodName: "Status",
			Handler:    _Benchmark_Status_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StartMacroBenchmark",
			Handler:       _Benchmark_StartMacroBenchmark_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "macro_benchmark.proto",
}
