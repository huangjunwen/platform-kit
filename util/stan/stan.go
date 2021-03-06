package stanutil

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	"github.com/rs/xid"
)

var (
	ErrNotConnected   = errors.New("stanutil: not yet connected to streaming server")
	ErrEmptyGroupName = errors.New("stanutil: empty group name")
)

// Conn 是对 stan.Conn 的封装；提供自动重连/重新订阅功能; 支持 Publish/PublishAsync 以及
// 持久化的 QueueSubscribe，不支持 Unsubscribe
type Conn struct {
	id        string
	options   Options
	connectMu sync.Mutex

	mu      sync.RWMutex
	sc      stan.Conn                   // sc 为空时表示连接未准备好
	stalech chan struct{}               // stalech 用于通知其它 routine sc 已过期失效
	subs    map[[2]string]*subscription // (subject, group)->subscription
	closed  bool                        // 是否已关闭
}

// subscription 是单个订阅
type subscription struct {
	conn *Conn

	// 基本属性
	subject string
	group   string
	cb      stan.MsgHandler

	// options
	options SubscriptionOptions
}

// NewConn 建立一个新的连接，nc 为 nats connection，必须设置 MaxReconnects(-1)，
// 即支持自动无限重连
func NewConn(nc *nats.Conn, clusterID string, opts ...Option) *Conn {

	c := &Conn{
		id:      xid.New().String(),
		options: NewOptions(),
		subs:    make(map[[2]string]*subscription),
	}
	for _, opt := range opts {
		opt(&c.options)
	}

	// 给日志添加上 id
	c.options.logger = c.options.logger.With().Str("client_id", c.id).Logger()

	var (
		connect func(bool)
		logger  = c.options.logger.With().Str("comp", "stanutil.Conn.connect").Logger()
	)

	// 该函数关闭旧连接（如果有的话），释放资源，并创建新连接，
	// 重新订阅
	connect = func(wait bool) {
		// 稍微停顿一下
		if wait {
			time.Sleep(c.options.reconnectWait)
		}

		// 保证同一时间只有一个 connect 在运行
		c.connectMu.Lock()
		defer c.connectMu.Unlock()

		// 置空字段，保证旧资源关闭并释放
		{
			c.mu.Lock()
			sc := c.sc
			stalech := c.stalech
			closed := c.closed
			c.sc = nil
			c.stalech = nil
			c.mu.Unlock()

			// 关闭连接并释放资源
			if sc != nil {
				sc.Close()
			}
			if stalech != nil {
				close(stalech)
			}

			// 已经关闭了
			if closed {
				logger.Info().Msg("Conn closed. connect aborted.")
				return
			}

		}

		// 开始连接
		opts := []stan.Option{}
		opts = append(opts, c.options.stanOptions...)
		opts = append(opts, stan.NatsConn(nc))
		opts = append(opts, stan.SetConnectionLostHandler(func(_ stan.Conn, _ error) {
			go connect(true)
		}))

		sc, err := stan.Connect(clusterID, c.id, opts...)
		if err != nil {
			// 失败了，继续重试
			logger.Error().Err(err).Msg("Failed to connect.")
			go connect(true)
			return
		}
		logger.Info().Msg("Connected.")

		// 成功了，需要更新字段，并重新订阅
		c.mu.Lock()
		defer c.mu.Unlock()

		c.sc = sc
		c.stalech = make(chan struct{})

		for _, sub := range c.subs {
			go sub.queueSubscribeTo(c.sc, c.stalech)
		}

		return

	}

	go connect(false)
	return c
}

// Close 关闭连接，释放资源
func (c *Conn) Close() {
	// 保证没有 connect 在运行
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	// 置空字段，保证旧资源关闭并释放，设 closed 为真
	c.mu.Lock()
	sc := c.sc
	stalech := c.stalech
	c.sc = nil
	c.stalech = nil
	c.closed = true
	c.mu.Unlock()

	// 关闭连接并释放资源
	if sc != nil {
		// NOTE: The callback (SetConnectionLostHandler) will not be invoked on normal Conn.Close().
		sc.Close()
	}
	if stalech != nil {
		close(stalech)
	}

	return
}

// Publish 同步发布消息，等同于 stan.Conn.Publish
func (c *Conn) Publish(subject string, data []byte) error {
	c.mu.RLock()
	sc := c.sc
	c.mu.RUnlock()
	if sc == nil {
		return ErrNotConnected
	}
	return sc.Publish(subject, data)
}

// PublishAsync 异步发布消息，等同于 stan.Conn.PublishAsync
func (c *Conn) PublishAsync(subject string, data []byte, ah stan.AckHandler) (string, error) {
	c.mu.RLock()
	sc := c.sc
	c.mu.RUnlock()
	if sc == nil {
		return "", ErrNotConnected
	}
	return sc.PublishAsync(subject, data, ah)
}

// QueueSubscribe 订阅一个主题：subject，group 为订阅的组别，必须非空；
// 该订阅是持久化的，该函数只会在参数错误或重复订阅时返回错误，网络类型的错误会重试
func (c *Conn) QueueSubscribe(subject, group string, cb stan.MsgHandler, opts ...SubscriptionOption) error {
	if group == "" {
		return ErrEmptyGroupName
	}

	sub := &subscription{
		conn:    c,
		subject: subject,
		group:   group,
		cb:      cb,
		options: NewSubscriptionOptions(),
	}
	for _, opt := range opts {
		opt(&sub.options)
	}

	// 检查是否有重复订阅
	key := [2]string{subject, group}
	c.mu.Lock()
	if c.subs[key] != nil {
		c.mu.Unlock()
		return fmt.Errorf("stanutil: subject=%+q group=%+q has already subscribed", subject, group)
	}
	c.subs[key] = sub
	sc := c.sc
	stalech := c.stalech
	c.mu.Unlock()

	// 若有连接，则发起订阅
	if sc != nil {
		go sub.queueSubscribeTo(sc, stalech)
	}
	return nil

}

func (sub *subscription) queueSubscribeTo(sc stan.Conn, stalech chan struct{}) {
	logger := sub.conn.options.logger.With().
		Str("comp", "stanutil.Conn.queueSubscribeTo").
		Str("subj", sub.subject).
		Str("grp", sub.group).Logger()

	// 给选项加上 DurableName
	opts := []stan.SubscriptionOption{}
	opts = append(opts, sub.options.stanOptions...)
	opts = append(opts, stan.DurableName(sub.group))

	// 如果订阅失败会一直重试，除非 sc 失效过期了
	stale := false
	for !stale {
		// 不支持 Unsubscribe，这样实现起来就比较简单了，不需要记录下来 stan.Subscription
		_, err := sc.QueueSubscribe(sub.subject, sub.group, sub.cb, opts...)
		if err == nil {
			logger.Info().Msg("Subscribed.")
			return
		}
		logger.Error().Err(err).Msg("Subscribe error.")

		// 等待一段时间
		t := time.NewTimer(sub.options.resubscribeWait)
		select {
		case <-stalech:
			stale = true
		case <-t.C:
		}
		t.Stop()
	}
}
