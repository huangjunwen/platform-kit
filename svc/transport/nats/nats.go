package natstransport

import (
	libsvc "bitbucket.org/jayven/platform-kit/svc"
	"bytes"
	"context"
	"errors"
	"github.com/nats-io/go-nats"
	"io"
	"math/rand"
	"sync"
)

type natsServer struct {
	errHandler func(error)
	mu         sync.Mutex
	conns      []*nats.Conn
	// svc name -> 在各个连接上的订阅
	subs map[string][]*nats.Subscription
}

type natsClient struct {
	mu    sync.RWMutex
	conns []*nats.Conn
}

type natsRequestor struct {
	conn    *nats.Conn
	svcName string
}

const (
	subjectPrefix = "svc."
	group         = "svc"
)

var (
	errNoConns      = errors.New("No nats.Conn")
	errConnNil      = errors.New("nats.Conn is nil")
	errServerClosed = errors.New("Server has closed")
	errClientClosed = errors.New("Client has closed")
)

// NewServer 使用 nats.Conn(s) 创建一个 RPCTransportServer；errHandler 用于处理内部错误，例如记录日志
func NewServer(conns []*nats.Conn, errHandler func(error)) libsvc.RPCTransportServer {
	if len(conns) == 0 {
		panic(errNoConns)
	}
	for _, conn := range conns {
		if conn == nil {
			panic(errConnNil)
		}
	}
	if errHandler == nil {
		errHandler = func(error) {}
	}

	return &natsServer{
		errHandler: errHandler,
		conns:      conns,
		subs:       make(map[string][]*nats.Subscription),
	}
}

func (server *natsServer) Register(svcName string, handler libsvc.RPCTransportHandler) error {
	server.mu.Lock()
	defer server.mu.Unlock()

	if len(server.conns) == 0 {
		return errServerClosed
	}

	if len(server.subs[svcName]) != 0 {
		return libsvc.ErrSvcNameConflict
	}

	subs := []*nats.Subscription{}
	for _, conn := range server.conns {
		conn := conn
		// 订阅到同一个 group 中以实现自动负载均衡
		sub, err := conn.QueueSubscribe(
			subj(svcName),
			group,
			func(reqMsg *nats.Msg) {
				go func() {
					reqReader := bytes.NewBuffer(reqMsg.Data)
					respWriter := &bytes.Buffer{}
					err := handler.Invoke(context.Background(), reqReader, respWriter)
					if err != nil {
						server.errHandler(err)
					}
					conn.Publish(reqMsg.Reply, respWriter.Bytes())
				}()
			},
		)
		if err != nil {
			// 出错了，将前面订阅过的都退订
			for _, sub := range subs {
				sub.Unsubscribe()
			}
			return err
		}
		subs = append(subs, sub)
	}

	server.subs[svcName] = subs
	return nil

}

func (server *natsServer) Deregister(svcName string) error {
	server.mu.Lock()
	defer server.mu.Unlock()

	if len(server.conns) == 0 {
		return errServerClosed
	}

	// 不存在
	subs := server.subs[svcName]
	if subs == nil {
		return nil
	}

	// 退订
	for _, sub := range subs {
		sub.Unsubscribe()
	}
	delete(server.subs, svcName)
	return nil

}

func (server *natsServer) Close() {
	server.mu.Lock()
	defer server.mu.Unlock()

	if len(server.conns) == 0 {
		panic(errServerClosed)
	}
	defer func() {
		server.conns = nil
	}()

	for _, subs := range server.subs {
		for _, sub := range subs {
			sub.Unsubscribe()
		}
	}
	server.subs = nil

}

// NewClient 使用 nats.Conn(s) 创建一个 RPCTransportClient
func NewClient(conns []*nats.Conn) libsvc.RPCTransportClient {
	if len(conns) == 0 {
		panic(errNoConns)
	}
	for _, conn := range conns {
		if conn == nil {
			panic(errConnNil)
		}
	}
	return &natsClient{
		conns: conns,
	}
}

func (client *natsClient) Discover(ctx context.Context, svcName string) (requestor libsvc.RPCTransportRequestor, err error) {
	client.mu.RLock()
	if len(client.conns) == 0 {
		client.mu.RUnlock()
		return nil, errClientClosed
	}
	conn := client.conns[rand.Intn(len(client.conns))]
	client.mu.RUnlock()

	return &natsRequestor{
		conn:    conn,
		svcName: svcName,
	}, nil
}

func (client *natsClient) Close() {
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.conns) == 0 {
		panic(errClientClosed)
	}
	client.conns = nil
}

func (requestor *natsRequestor) Invoke(ctx context.Context, writeReq func(io.Writer) error) (respReader io.Reader, err error) {
	reqWriter := &bytes.Buffer{}
	if err := writeReq(reqWriter); err != nil {
		return nil, err
	}
	respMsg, err := requestor.conn.RequestWithContext(ctx, subj(requestor.svcName), reqWriter.Bytes())
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(respMsg.Data), nil
}

func subj(name string) string {
	return subjectPrefix + name
}
