// Code generated from ScalarDL 3.12 proto definitions. DO NOT EDIT.
// Source: scalar.proto (scalar-labs/scalardl)
// Generated gRPC service bindings for the Tegata audit layer.
// Full proto source: https://github.com/scalar-labs/scalardl/blob/master/rpc/src/main/proto/scalar.proto

package rpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// LedgerClient is the client API for the Ledger service.
// It exposes only the ExecuteContract RPC used by the Tegata audit layer.
// The underlying gRPC ClientConn is shared with LedgerPrivilegedClient.
type LedgerClient interface {
	ExecuteContract(ctx context.Context, in *ContractExecutionRequest, opts ...grpc.CallOption) (*ContractExecutionResponse, error)
}

type ledgerClient struct {
	cc grpc.ClientConnInterface
}

// NewLedgerClient creates a new LedgerClient bound to the provided connection.
// The same grpc.ClientConn should also be passed to NewLedgerPrivilegedClient.
func NewLedgerClient(cc grpc.ClientConnInterface) LedgerClient {
	return &ledgerClient{cc}
}

func (c *ledgerClient) ExecuteContract(ctx context.Context, in *ContractExecutionRequest, opts ...grpc.CallOption) (*ContractExecutionResponse, error) {
	out := new(ContractExecutionResponse)
	err := c.cc.Invoke(ctx, "/rpc.Ledger/ExecuteContract", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LedgerPrivilegedClient is the client API for the LedgerPrivileged service.
// It provides RegisterCert to associate a client certificate with an entity.
// Listens on port 50052 (separate from the Ledger service on 50051).
type LedgerPrivilegedClient interface {
	RegisterCert(ctx context.Context, in *CertificateRegistrationRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type ledgerPrivilegedClient struct {
	cc grpc.ClientConnInterface
}

// NewLedgerPrivilegedClient creates a new LedgerPrivilegedClient bound to the
// provided connection. The privileged service typically runs on port 50052.
func NewLedgerPrivilegedClient(cc grpc.ClientConnInterface) LedgerPrivilegedClient {
	return &ledgerPrivilegedClient{cc}
}

func (c *ledgerPrivilegedClient) RegisterCert(ctx context.Context, in *CertificateRegistrationRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/rpc.LedgerPrivileged/RegisterCert", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LedgerServer is the server API for the Ledger service.
// Included for completeness; Tegata does not implement a server.
type LedgerServer interface {
	ExecuteContract(context.Context, *ContractExecutionRequest) (*ContractExecutionResponse, error)
}

// UnimplementedLedgerServer provides a base implementation of LedgerServer
// that returns Unimplemented for all methods.
type UnimplementedLedgerServer struct{}

func (UnimplementedLedgerServer) ExecuteContract(context.Context, *ContractExecutionRequest) (*ContractExecutionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ExecuteContract not implemented")
}

// LedgerPrivilegedServer is the server API for the LedgerPrivileged service.
type LedgerPrivilegedServer interface {
	RegisterCert(context.Context, *CertificateRegistrationRequest) (*emptypb.Empty, error)
}

// UnimplementedLedgerPrivilegedServer provides a base implementation of
// LedgerPrivilegedServer that returns Unimplemented for all methods.
type UnimplementedLedgerPrivilegedServer struct{}

func (UnimplementedLedgerPrivilegedServer) RegisterCert(context.Context, *CertificateRegistrationRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterCert not implemented")
}

// RegisterLedgerServer registers the LedgerServer on the gRPC server.
func RegisterLedgerServer(s grpc.ServiceRegistrar, srv LedgerServer) {
	s.RegisterService(&Ledger_ServiceDesc, srv)
}

// RegisterLedgerPrivilegedServer registers the LedgerPrivilegedServer on the gRPC server.
func RegisterLedgerPrivilegedServer(s grpc.ServiceRegistrar, srv LedgerPrivilegedServer) {
	s.RegisterService(&LedgerPrivileged_ServiceDesc, srv)
}

// Ledger_ServiceDesc is the grpc.ServiceDesc for the Ledger service.
var Ledger_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "rpc.Ledger",
	HandlerType: (*LedgerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ExecuteContract",
			Handler:    _Ledger_ExecuteContract_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "scalar.proto",
}

func _Ledger_ExecuteContract_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ContractExecutionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LedgerServer).ExecuteContract(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpc.Ledger/ExecuteContract",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LedgerServer).ExecuteContract(ctx, req.(*ContractExecutionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// LedgerPrivileged_ServiceDesc is the grpc.ServiceDesc for the LedgerPrivileged service.
var LedgerPrivileged_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "rpc.LedgerPrivileged",
	HandlerType: (*LedgerPrivilegedServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterCert",
			Handler:    _LedgerPrivileged_RegisterCert_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "scalar.proto",
}

func _LedgerPrivileged_RegisterCert_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CertificateRegistrationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LedgerPrivilegedServer).RegisterCert(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rpc.LedgerPrivileged/RegisterCert",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LedgerPrivilegedServer).RegisterCert(ctx, req.(*CertificateRegistrationRequest))
	}
	return interceptor(ctx, in, info, handler)
}
