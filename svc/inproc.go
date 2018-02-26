package libsvc

import (
	"context"
	"sync"
)

type inprocServer struct{}

type inprocClient struct{}

type inprocClientService struct {
	name string
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
	_ Service       = (*inprocClientService)(nil)
)

// InprocServer 创建一个进程内服务端，在此服务端注册的服务只能被同进程内的客户端访问到
func InprocServer() ServiceServer {
	return defaultInprocServer
}

// InprocClient 创建一个进程内客户端，可以访问同进程内的注册的服务
func InprocClient() ServiceClient {
	return defaultInprocClient
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
	return s.Invoke(ctx, method, input, output)

}
