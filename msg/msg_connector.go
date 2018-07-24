package libmsg

import (
	"fmt"
	"sync"
	"time"

	stan "github.com/nats-io/go-nats-streaming"
)

var (
	// 默认批量消息大小
	DefaultOptBatch = 500
	// 默认拉数据间隔
	DefaultOptFetchInterval = 30 * time.Second
)

// MsgEntry 代表一个将要发布的消息条目
type MsgEntry interface {
	// Subject 返回需要发布的主题
	Subject() string

	// Data 返回数据
	Data() []byte
}

// MsgStore 代表消息仓库
type MsgStore interface {
	// Fetch 从 MsgSource 提取有要发布的消息
	Fetch() <-chan MsgEntry

	// ProcessResult 在发布后调用，results[i] 表示 msgs[i] 的发布结果:
	// true 为发布成功，false 为发布失败，需要重试
	// NOTE: msgs 长度不会超过 MsgConnector 的批量数目
	ProcessResult(msgs []MsgEntry, results []bool)
}

// MsgConnector 用于将 MsgStore 中的消息发布到 nats-streaming-server 上.
// At-least-once：即 MsgStore 中的消息最少会被发送一次，但有可能会重复发；因此接收方必须保证幂等性
type MsgConnector struct {
	sc     stan.Conn
	store  MsgStore
	kickch chan struct{}
	stopch chan struct{}

	// options
	batch         int
	fetchInterval time.Duration
}

// MsgConnectorOption 是用于创建 MsgConnector 的配置
type MsgConnectorOption func(*MsgConnector) error

// OptBatch 设置一次批量发送消息的数量，默认 500
func OptBatch(batch int) MsgConnectorOption {
	return func(c *MsgConnector) error {
		if batch <= 0 {
			return fmt.Errorf("batch <= 0")
		}
		c.batch = batch
		return nil
	}
}

// OptFetchInterval 设置主动从 MsgSource 中拉数据的时间间隔，默认 30 秒
func OptFetchInterval(fetchInterval time.Duration) MsgConnectorOption {
	return func(c *MsgConnector) error {
		c.fetchInterval = fetchInterval
		return nil
	}
}

// NewMsgConnector 创建一个 MsgConnector
func NewMsgConnector(sc stan.Conn, src MsgStore, opts ...MsgConnectorOption) (*MsgConnector, error) {
	ret := &MsgConnector{
		sc:            sc,
		store:         src,
		kickch:        make(chan struct{}, 1),
		stopch:        make(chan struct{}),
		batch:         DefaultOptBatch,
		fetchInterval: DefaultOptFetchInterval,
	}

	for _, opt := range opts {
		if err := opt(ret); err != nil {
			return nil, err
		}
	}

	go ret.loop()
	return ret, nil
}

func (c *MsgConnector) loop() {

	stopped := false
	for !stopped {

		// 抓取要发送的消息
		msgch := c.store.Fetch()
		for {
			// 一次从 msgch 中抓取不超过 batch 的消息
			msgs := []MsgEntry{}
			results := []bool{}
			for msg := range msgch {
				msgs = append(msgs, msg)
				results = append(results, false)
				if len(msgs) >= c.batch {
					break
				}
			}
			if len(msgs) == 0 {
				// 没有消息了
				break
			}

			// 开始发送
			var (
				id2Msg  = make(map[string]int)
				wg      sync.WaitGroup
				mu      sync.Mutex
				success = make(map[string]struct{}) // 成功集合
			)

			// 添加 counter
			wg.Add(len(msgs))

			ackHandler := func(id string, err error) {
				// 添加到成功集合中
				if err == nil {
					mu.Lock()
					success[id] = struct{}{}
					mu.Unlock()
				}

				// 减少 counter
				wg.Done()
			}

			// 批量发送
			nErrs := 0
			for i, msg := range msgs {
				id, err := c.sc.PublishAsync(msg.Subject(), msg.Data(), ackHandler)
				if err != nil {
					nErrs += 1
				} else {
					id2Msg[id] = i
				}
			}

			// 减去直接失败的
			if nErrs > 0 {
				wg.Add(-nErrs)
			}

			// 等待完成
			wg.Wait()

			// 处理 success
			for id, _ := range success {
				results[id2Msg[id]] = true
			}

			// 通知 MsgSource
			c.store.ProcessResult(msgs, results)

		}

		// 间隔一段时间也主动 fetch 一次
		t := time.NewTimer(c.fetchInterval)

		select {
		case <-c.stopch:
			stopped = true
		case <-t.C:
		case <-c.kickch:
		}

		t.Stop()

	}

}

// Kick 让 connector 立即从 MsgStore 中拉取消息发布
func (c *MsgConnector) Kick() {
	// 非阻塞地往 kickch 发送消息
	select {
	case c.kickch <- struct{}{}:
	default:
	}
}

// Stop 停止 connector
func (c *MsgConnector) Stop() {
	close(c.stopch)
}
