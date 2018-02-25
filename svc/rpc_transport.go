package libsvc

import (
	"context"
	"io"
)

// RPCTransportServer 代表一个传输层面的 RPC 服务端，负责服务注册以及接收请求数据/返回响应数据
type RPCTransportServer interface {
	// Register 注册一个名为 svcName 的服务，注册成功后 RPCTransportClient 可以对其发起请求，
	// 多次注册同名服务必须报错
	Register(svcName string, handler RPCTransportHandler) error

	// Deregister 取消注册名为 svcName 的服务
	Deregister(svcName string) error

	// Close 释放资源，包括已注册的服务
	Close()
}

// RPCTransportClient 代表一个传输层面的 RPC 客户端，负责服务发现以及发送请求数据/接收响应数据
type RPCTransportClient interface {
	// Discover 发现一个名为 svcName 的服务，若成功则可以使用返回的请求器发起请求
	Discover(ctx context.Context, svcName string) (requestor RPCTransportRequestor, err error)

	// Close 释放资源
	Close()
}

// RPCTransportHandler 代表一个传输层面的处理器
type RPCTransportHandler interface {
	// Invoke 从 reqReader 读取请求数据，往 respWriter 写入响应数据交由 RPCTransportServer 返回给客户端
	Invoke(ctx context.Context, reqReader io.Reader, respWriter io.Writer) error
}

// RPCTransportHandlerFunc 适配 RPCTransportHandler
type RPCTransportHandlerFunc func(ctx context.Context, reqReader io.Reader, respWriter io.Writer) error

// Invoke 实现 RPCTransportHandler 接口
func (fn RPCTransportHandlerFunc) Invoke(ctx context.Context, reqReader io.Reader, respWriter io.Writer) error {
	return fn(ctx, reqReader, respWriter)
}

// RPCTransportRequestor 代表一个传输层面的请求器
type RPCTransportRequestor interface {
	// Invoke 发送请求并等待响应，调用者应该提供一个 writeReq 函数用于写入请求
	Invoke(ctx context.Context, writeReq func(reqWriter io.Writer) error) (respReader io.Reader, err error)
}
