package libsvc

import (
	"context"
)

// ServiceHandler 是 Service.Invoke 的函数原型
type ServiceHandler func(context.Context, Method, interface{}, interface{}) error

// ServiceMiddleware 是 ServiceHandler 的中间件
type ServiceMiddleware func(ServiceHandler) ServiceHandler

type decSvc struct {
	svc Service
	h   ServiceHandler
}

type decSvcWithItf struct {
	svc ServiceWithInterface
	h   ServiceHandler
}

type decClient struct {
	client ServiceClient
	mws    []ServiceMiddleware
}

type decServer struct {
	server ServiceServer
	mws    []ServiceMiddleware
}

// DecorateService 为 Service 添加中间件，mws[0] 是最外层中间件
func DecorateService(svc Service, mws ...ServiceMiddleware) Service {
	h := svc.Invoke
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return &decSvc{
		svc: svc,
		h:   h,
	}
}

// DecorateServiceWithInterface 为 ServiceWithInterface 添加中间件，mws[0] 是最外层中间件
func DecorateServiceWithInterface(svc ServiceWithInterface, mws ...ServiceMiddleware) ServiceWithInterface {
	h := svc.Invoke
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return &decSvcWithItf{
		svc: svc,
		h:   h,
	}
}

// DecorateClient 返回装饰过的 ServiceClient，由该 client make 出来的 Service 都会安装上 mws 中间件，
// mws[0] 是最外层中间件
func DecorateClient(client ServiceClient, mws ...ServiceMiddleware) ServiceClient {
	return &decClient{
		client: client,
		mws:    mws,
	}
}

// DecorateServer 返回装饰过的 ServiceServer，往该 server 注册的 Service 都会安装上 mws 中间件，
// mws[0] 是最外层中间件
func DecorateServer(server ServiceServer, mws ...ServiceMiddleware) ServiceServer {
	return &decServer{
		server: server,
		mws:    mws,
	}
}

// Name 实现 Service 接口
func (svc *decSvc) Name() string {
	return svc.svc.Name()
}

// Invoke 实现 Service 接口
func (svc *decSvc) Invoke(ctx context.Context, method Method, input, output interface{}) error {
	return svc.h(ctx, method, input, output)
}

// Name 实现 Service 接口
func (svc *decSvcWithItf) Name() string {
	return svc.svc.Name()
}

// Invoke 实现 Service 接口
func (svc *decSvcWithItf) Invoke(ctx context.Context, method Method, input, output interface{}) error {
	return svc.h(ctx, method, input, output)
}

// Interface 实现 ServiceWithInterface 接口
func (svc *decSvcWithItf) Interface() Interface {
	return svc.svc.Interface()
}

// Make 实现 ServiceClient 接口
func (client *decClient) Make(svcName string) Service {
	svc := client.client.Make(svcName)
	return DecorateService(svc, client.mws...)
}

// Register 实现 ServiceServer 接口
func (server *decServer) Register(svc ServiceWithInterface) error {
	return server.server.Register(DecorateServiceWithInterface(svc, server.mws...))
}

// Deregister 实现 ServiceServer 接口
func (server *decServer) Deregister(svcName string) error {
	return server.server.Deregister(svcName)
}
