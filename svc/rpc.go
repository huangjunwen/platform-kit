package libsvc

import (
	"context"
	"io"
)

type rpcServer struct {
	protocol  RPCServerProtocolFactory
	transport RPCTransportServer
}

type rpcClient struct {
	protocol  RPCClientProtocolFactory
	transport RPCTransportClient
}

type rpcClientService struct {
	name   string
	client *rpcClient
}

var (
	_ ServiceServer = (*rpcServer)(nil)
	_ ServiceClient = (*rpcClient)(nil)
	_ Service       = (*rpcClientService)(nil)
)

// NewRPCServer 创建一个 RPC 服务端，在此注册的服务可以被对应的 RPCClient 访问
func NewRPCServer(protocol RPCServerProtocolFactory, transport RPCTransportServer) ServiceServer {
	return &rpcServer{
		protocol:  protocol,
		transport: transport,
	}
}

func (server *rpcServer) Register(svc ServiceWithInterface) error {
	itf := svc.Interface()
	return server.transport.Register(
		svc.Name(),
		RPCTransportHandlerFunc(func(ctx context.Context, reqReader io.Reader, respWriter io.Writer) error {
			protocol := server.protocol.Protocol()

			// 解析出方法名和 passthru
			done, methodName, passthru, err := protocol.ProcessRequest(respWriter, reqReader)
			if err != nil || done {
				return err
			}

			// 查找方法
			method := itf.MethodByName(methodName)

			// 找不到
			if method == nil {
				return protocol.ProcessMethodNotFound(respWriter, methodName)
			}

			// 入参
			input := method.GenInput()
			done, err = protocol.ProcessInput(respWriter, input)
			if err != nil || done {
				return err
			}

			// 执行
			if len(passthru) != 0 {
				ctx = WithPassthru(ctx, passthru)
			}
			output, outputErr := svc.Invoke(ctx, method, input)
			// NOTE: svc.Invoke 应该已经检查 output 的类型，所以这里不用再检查了

			// 出参
			return protocol.ProcessOutput(respWriter, output, outputErr)
		}),
	)

}

func (server *rpcServer) Deregister(svcName string) error {
	return server.transport.Deregister(svcName)
}

// NewRPCClient 创建一个 RPC 客户端，可以用于访问远程服务
func NewRPCClient(protocol RPCClientProtocolFactory, transport RPCTransportClient) ServiceClient {
	return &rpcClient{
		protocol:  protocol,
		transport: transport,
	}
}

func (client *rpcClient) Make(svcName string) Service {
	if !IsValidServiceName(svcName) {
		panic(ErrBadSvcName)
	}
	return &rpcClientService{
		name:   svcName,
		client: client,
	}
}

func (svc *rpcClientService) Name() string {
	return svc.name
}

func (svc *rpcClientService) Invoke(ctx context.Context, method Method, input interface{}) (interface{}, error) {
	// 首先检查一下 input type
	method.AssertInputType(input)

	client := svc.client
	protocol := client.protocol.Protocol()

	// 发现服务
	requestor, err := client.transport.Discover(ctx, svc.name)
	if err != nil {
		return nil, err
	}

	// 远程调用
	respReader, err := requestor.Invoke(ctx, func(reqWriter io.Writer) error {
		// 入参 -> RPC 请求
		return protocol.ProcessInput(reqWriter, method.Name(), input, Passthru(ctx))
	})
	if err != nil {
		return nil, err
	}

	// RPC 响应 -> 出参
	output := method.GenOutput()
	err = protocol.ProcessOutput(respReader, output)
	if err != nil {
		return nil, err
	}

	return output, nil
}
