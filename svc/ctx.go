package libsvc

import (
	"context"
)

type passthruKeyType struct{}

// proxyContext 只允许该 key 通过
var passthruKey = passthruKeyType{}

// Passthru 从 Context 中提取 Passthru 字典
func Passthru(ctx context.Context) map[string]string {
	v := ctx.Value(passthruKey)
	if v == nil {
		return nil
	}
	return v.(map[string]string)
}

// WithPassthru 给 Context 添加 Passthru 字典，用于在 Service 调用过程中传递一些
// 上下文信息（参数不应该放在这里）
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
