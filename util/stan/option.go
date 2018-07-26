package stanutil

import (
	"time"

	"github.com/nats-io/go-nats-streaming"
)

var (
	// 默认重连间隔
	DefaultReconnectWait = 5 * time.Second
	// 默认重新订阅间隔
	DefaultResubscribeWait = 5 * time.Second
)

// Options 是连接选项
type Options struct {
	stanOptions []stan.Option

	// reconnectWait 重连间隔时间
	reconnectWait time.Duration
}

// SubscriptionOptions 是订阅选项
type SubscriptionOptions struct {
	stanOptions []stan.SubscriptionOption

	// resubscribeWait 重新订阅间隔时间
	resubscribeWait time.Duration
}

// Option 是单个连接选项
type Option func(*Options)

// SubscriptionOption 是单个订阅选项
type SubscriptionOption func(*SubscriptionOptions)

// NewOptions 返回默认 Options
func NewOptions() Options {
	return Options{
		stanOptions:   []stan.Option{},
		reconnectWait: DefaultReconnectWait,
	}
}

// OptConnectWait 设置连接超时
func OptConnectWait(t time.Duration) Option {
	return func(o *Options) {
		o.stanOptions = append(o.stanOptions, stan.ConnectWait(t))
	}
}

// OptPubAckWait 设置 publish 的 ack 超时
func OptPubAckWait(t time.Duration) Option {
	return func(o *Options) {
		o.stanOptions = append(o.stanOptions, stan.PubAckWait(t))
	}
}

// OptPings 设置 ping
func OptPings(interval, maxOut int) Option {
	return func(o *Options) {
		o.stanOptions = append(o.stanOptions, stan.Pings(interval, maxOut))
	}
}

// OptReconnectWait 设置重连时间间隔
func OptReconnectWait(t time.Duration) Option {
	return func(o *Options) {
		o.reconnectWait = t
	}
}

// NewSubscriptionOptions 返回默认订阅选项
func NewSubscriptionOptions() SubscriptionOptions {
	return SubscriptionOptions{
		stanOptions:     []stan.SubscriptionOption{},
		resubscribeWait: DefaultResubscribeWait,
	}
}

// SubsOptMaxInflight 设置服务器在获得 ack 之前最多发送多少消息给 subscriber
func SubsOptMaxInflight(m int) SubscriptionOption {
	return func(o *SubscriptionOptions) {
		o.stanOptions = append(o.stanOptions, stan.MaxInflight(m))
	}
}

// SubsOptAckWait 设置服务器等待 subscriber ack 的超时时间
func SubsOptAckWait(t time.Duration) SubscriptionOption {
	return func(o *SubscriptionOptions) {
		o.stanOptions = append(o.stanOptions, stan.AckWait(t))
	}
}

// SubsOptManualAcks 设置手动 ack 模式
func SubsOptManualAcks() SubscriptionOption {
	return func(o *SubscriptionOptions) {
		o.stanOptions = append(o.stanOptions, stan.SetManualAckMode())
	}
}

// SubsOptResubscribeWait 设置重新订阅间隔
func SubsOptResubscribeWait(t time.Duration) SubscriptionOption {
	return func(o *SubscriptionOptions) {
		o.resubscribeWait = t
	}
}
