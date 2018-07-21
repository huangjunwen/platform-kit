package libsvc

import (
	"context"
	"regexp"
)

// Service 代表一个可供调用的服务
type Service interface {
	// Name 返回服务名称
	Name() string

	// Invoke 调用服务的一个方法，input/output 必须满足 method 定义的类型，否则 Invoke 应当 panic；
	// NOTE: 仅当 error 为 nil 时，output 有效
	Invoke(ctx context.Context, method Method, input, output interface{}) error
}

// ServiceWithInterface 代表一个绑定了接口定义的服务
//
// NOTE: 如同 golang 中的一样，这里所绑定的接口可以只是该服务的全部方法的一个子集
type ServiceWithInterface interface {
	// 首先是一个 Service
	Service

	// Interface 返回该服务实现的所有方法的一个子集（包括全集）
	Interface() Interface
}

// ServiceServer 代表服务的服务端一方，负责将服务注册起来以供调用
type ServiceServer interface {
	// Register 注册一个服务使得客户端可以调用，只有接口中定义的方法可被调用，
	// 多次注册同名服务必须报错
	//
	// NOTE: 之所以这里使用 ServiceWithInterface 而不只是 Service 是因为服务端很可能需要完整的方法信息，
	// 例如若是 rpc 服务器，需要从方法的名称获得对应方法定义；又如服务端可能需要对外发布所有方法的定义
	Register(svc ServiceWithInterface) error

	// Deregister 取消注册名为 svcName 的服务，如果没有这个服务，不必报错
	Deregister(svcName string) error
}

// ServiceClient 代表服务的客户端一方，负责发起调用
type ServiceClient interface {
	// Make 创建一个名为 svcName 的远程服务
	Make(svcName string) Service
}

type boundService struct {
	Service
	itf Interface
}

type localService struct {
	name     string
	methods  map[string]Method
	handlers map[Method]MethodHandler
}

var (
	_ ServiceWithInterface = (*boundService)(nil)
	_ ServiceWithInterface = (*localService)(nil)
)

// BindInterface 绑定一个 Interface (itf) 到指定 Service (svc) 上，若 svc
// 本身已经是 ServiceWithInterface，则 itf 必须是当前接口的子集，否则会 panic
func BindInterface(svc Service, itf Interface) ServiceWithInterface {
	s, ok := svc.(ServiceWithInterface)
	if ok {
		curItf := s.Interface()
		for _, m := range itf.Methods() {
			if !curItf.HasMethod(m) {
				panic(ErrMethodNotFound)
			}
		}
	}
	return &boundService{
		Service: svc,
		itf:     itf,
	}
}

func (svc *boundService) Interface() Interface {
	return svc.itf
}

// NewLocalService 新建一个本地服务，methodAndHandlers 应当为一系列 Method 和 MethodHandler/MethodHandlerFunc 对：
//   Method1, Handler1, Method2, Handler2, ...
func NewLocalService(svcName string, methodAndHandlers ...interface{}) ServiceWithInterface {
	if !IsValidServiceName(svcName) {
		panic(ErrBadSvcName)
	}
	svc := &localService{
		name:     svcName,
		methods:  make(map[string]Method),
		handlers: make(map[Method]MethodHandler),
	}
	// 应当偶数个
	if len(methodAndHandlers)&1 == 1 {
		panic(ErrMethodHandlerPair)
	}
	for i := 0; i < len(methodAndHandlers); i += 2 {
		var (
			method  Method
			handler MethodHandler
			ok      bool
		)

		// 检查 method
		if method, ok = methodAndHandlers[i].(Method); !ok {
			panic(ErrMethodHandlerPair)
		}
		// 检查 handler
		switch h := methodAndHandlers[i+1].(type) {
		case func(context.Context, interface{}, interface{}) error:
			handler = MethodHandlerFunc(h)
		case MethodHandler:
			handler = h
		default:
			panic(ErrMethodHandlerPair)
		}
		// 存
		svc.methods[method.Name()] = method
		svc.handlers[method] = handler
	}
	return svc
}

func (svc *localService) Name() string {
	return svc.name
}

func (svc *localService) Invoke(ctx context.Context, method Method, input, output interface{}) error {
	// 查找方法
	handler := svc.handlers[method]
	if handler == nil {
		return ErrMethodNotFound
	}

	// 对出入参进行类型检查
	method.AssertInputType(input)
	method.AssertOutputType(output)

	// 执行 handler
	return handler.Invoke(ctx, input, output)

}

func (svc *localService) Interface() Interface {
	return defaultInterface(svc.methods)
}

var (
	serviceNameRegexp = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_]*)(\.([a-zA-Z][a-zA-Z0-9_]*))*$`)
)

// IsValidServiceName 返回一个字符串是否是合法的服务名：xxx.xxx.xxx
// 其中 xxx 为合法的变量名，即字母开头，后续为字母数字或下划线
func IsValidServiceName(svcName string) bool {
	return serviceNameRegexp.MatchString(svcName)
}
