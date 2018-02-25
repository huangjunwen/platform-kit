package libsvc

import (
	"context"
	"time"
)

// proxyContext 是一个 Context 代理，本身实现 Context 接口；除
// Value 外其它方法皆是调用其所代理的 Context 对象；调用 Value
// 除非 key 是 passthruKey 外都返回 nil；如此一来可以建立一个“崭新”
// 的上下文环境
type proxyContext struct {
	ctx context.Context
}

type passthruKeyType struct{}

// proxyContext 只允许该 key 通过
var passthruKey = passthruKeyType{}

func newProxyContext(ctx context.Context) context.Context {
	return &proxyContext{ctx}
}

func (ctx *proxyContext) Deadline() (deadline time.Time, ok bool) {
	return ctx.ctx.Deadline()
}

func (ctx *proxyContext) Done() <-chan struct{} {
	return ctx.ctx.Done()
}

func (ctx *proxyContext) Err() error {
	return ctx.ctx.Err()
}

func (ctx *proxyContext) Value(key interface{}) interface{} {
	if key == passthruKey {
		return ctx.ctx.Value(passthruKey)
	}
	return nil
}

func Passthru(ctx context.Context) map[string]string {
	v := ctx.Value(passthruKey)
	if v == nil {
		return nil
	}
	return v.(map[string]string)
}

func WithPassthru(ctx context.Context, kv map[string]string) context.Context {
	p := Passthru(ctx)
	if p == nil {
		p = kv
	} else {
		for k, v := range kv {
			p[k] = v
		}
	}
	return context.WithValue(ctx, passthruKey, p)
}
