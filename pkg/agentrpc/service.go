package agentrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	AgentServiceName      = "swoops.AgentService"
	connectMethodFullName = "/swoops.AgentService/Connect"
)

type AgentServiceClient interface {
	Connect(ctx context.Context, opts ...grpc.CallOption) (AgentService_ConnectClient, error)
}

type agentServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAgentServiceClient(cc grpc.ClientConnInterface) AgentServiceClient {
	return &agentServiceClient{cc: cc}
}

func (c *agentServiceClient) Connect(ctx context.Context, opts ...grpc.CallOption) (AgentService_ConnectClient, error) {
	opts = append(opts, grpc.CallContentSubtype(jsonCodecName))
	stream, err := c.cc.NewStream(ctx, &AgentService_ServiceDesc.Streams[0], connectMethodFullName, opts...)
	if err != nil {
		return nil, err
	}
	return &agentServiceConnectClient{ClientStream: stream}, nil
}

type AgentService_ConnectClient interface {
	Send(*AgentEnvelope) error
	Recv() (*ControlEnvelope, error)
	grpc.ClientStream
}

type agentServiceConnectClient struct {
	grpc.ClientStream
}

func (c *agentServiceConnectClient) Send(msg *AgentEnvelope) error {
	return c.ClientStream.SendMsg(msg)
}

func (c *agentServiceConnectClient) Recv() (*ControlEnvelope, error) {
	m := new(ControlEnvelope)
	if err := c.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type AgentServiceServer interface {
	Connect(AgentService_ConnectServer) error
}

type UnimplementedAgentServiceServer struct{}

func (UnimplementedAgentServiceServer) Connect(AgentService_ConnectServer) error {
	return status.Error(codes.Unimplemented, "method Connect not implemented")
}

func RegisterAgentServiceServer(s grpc.ServiceRegistrar, srv AgentServiceServer) {
	s.RegisterService(&AgentService_ServiceDesc, srv)
}

type AgentService_ConnectServer interface {
	Send(*ControlEnvelope) error
	Recv() (*AgentEnvelope, error)
	grpc.ServerStream
}

type agentServiceConnectServer struct {
	grpc.ServerStream
}

func (s *agentServiceConnectServer) Send(msg *ControlEnvelope) error {
	return s.ServerStream.SendMsg(msg)
}

func (s *agentServiceConnectServer) Recv() (*AgentEnvelope, error) {
	m := new(AgentEnvelope)
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _AgentService_Connect_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(AgentServiceServer).Connect(&agentServiceConnectServer{ServerStream: stream})
}

var AgentService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: AgentServiceName,
	HandlerType: (*AgentServiceServer)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Connect",
			Handler:       _AgentService_Connect_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "proto/swoops/agent.proto",
}
