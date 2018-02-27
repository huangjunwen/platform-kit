package libsvc

import (
	"context"
	"sync"
)

type inprocServer struct{}

type inprocClient struct{}

type inprocFirstClient struct {
	alt ServiceClient
}

type inprocClientService struct {
	name string
}

type inprocFirstClientService struct {
	name string
	alt  Service
}

var (
	inproc = struct {
		mu   sync.RWMutex
		svcs map[string]ServiceWithInterface
	}{
		svcs: make(map[string]ServiceWithInterface),
	}
)

var (
	defaultInprocServer = inprocServer{}
	defaultInprocClient = inprocClient{}
)

var (
	_ ServiceServer = defaultInprocServer
	_ ServiceClient = defaultInprocClient
	_ ServiceClient = (*inprocFirstClient)(nil)
	_ Service       = (*inprocClientService)(nil)
	_ Service       = (*inprocFirstClientService)(nil)
)

// InprocServer 创建一个进程内服务端，在此服务端注册的服务只能被同进程内的客户端访问到
func InprocServer() ServiceServer {
	return defaultInprocServer
}

func (server inprocServer) Register(svc ServiceWithInterface) error {
	inproc.mu.Lock()
	defer inproc.mu.Unlock()

	if inproc.svcs[svc.Name()] != nil {
		return ErrSvcNameConflict
	}
	inproc.svcs[svc.Name()] = svc
	return nil
}

func (server inprocServer) Deregister(svcName string) error {
	inproc.mu.Lock()
	defer inproc.mu.Unlock()
	delete(inproc.svcs, svcName)
	return nil
}

// InprocClient 创建一个进程内客户端，可以访问同进程内的注册的服务
func InprocClient() ServiceClient {
	return defaultInprocClient
}

func (client inprocClient) Make(svcName string) Service {
	if !IsValidServiceName(svcName) {
		panic(ErrBadSvcName)
	}
	return &inprocClientService{
		name: svcName,
	}
}

func (svc *inprocClientService) Name() string {
	return svc.name
}

func (svc *inprocClientService) Invoke(ctx context.Context, method Method, input, output interface{}) error {
	inproc.mu.RLock()
	s := inproc.svcs[svc.name]
	inproc.mu.RUnlock()

	if s == nil {
		return ErrSvcNotFound
	}
	// 这里不需要再次检查 input/output 类型，因为接下来 s.Invoke 是会检查的
	return s.Invoke(ctx, method, input, output)

}

// NewInprocFirstClient 创建一个客户端，该客户端会首先寻找本进程内的服务，若找不到时会使用 alt
func NewInprocFirstClient(alt ServiceClient) ServiceClient {
	if alt == defaultInprocClient {
		panic(ErrAltIsInprocClient)
	}
	return &inprocFirstClient{
		alt: alt,
	}
}

func (client *inprocFirstClient) Make(svcName string) Service {
	if !IsValidServiceName(svcName) {
		panic(ErrBadSvcName)
	}
	return &inprocFirstClientService{
		name: svcName,
		alt:  client.alt.Make(svcName),
	}
}

func (svc *inprocFirstClientService) Name() string {
	return svc.name
}

func (svc *inprocFirstClientService) Invoke(ctx context.Context, method Method, input, output interface{}) error {
	inproc.mu.RLock()
	s := inproc.svcs[svc.name]
	inproc.mu.RUnlock()

	if s == nil {
		return svc.alt.Invoke(ctx, method, input, output)
	}
	return s.Invoke(ctx, method, input, output)
}
