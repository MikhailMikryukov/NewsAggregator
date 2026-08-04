package rabbitmq

import (
	"NewsAggregator/internal/config"
	"context"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultConnectionTimeout = 10 * time.Second
	defaultHeartBeat         = 10 * time.Second
)

type RabbitClient struct {
	cfg    config.RabbitConfig
	conn   *amqp091.Connection
	mu     sync.RWMutex
	notify chan *amqp091.Error
	ctx    context.Context
	cancel context.CancelFunc
	closed atomic.Bool
}

func NewClient(cfg config.RabbitConfig) (*RabbitClient, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	if cfg.ConnectionTimeout == 0 {
		cfg.ConnectionTimeout = defaultConnectionTimeout
	}

	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = defaultHeartBeat
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &RabbitClient{
		cfg:    cfg,
		conn:   nil,
		ctx:    ctx,
		cancel: cancel,
	}

	if err := client.connect(); err != nil {
		cancel()
		return nil, fmt.Errorf("initial rabbit connect err: %w", err)
	}

	return client, nil
}

func (c *RabbitClient) connect() error {
	dialer := net.Dialer{
		Timeout:   c.cfg.ConnectionTimeout,
		KeepAlive: c.cfg.Heartbeat,
	}

	amqpConfig := amqp091.Config{
		Heartbeat: c.cfg.Heartbeat,
		Locale:    "en_US",
		Dial: func(network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	conn, err := amqp091.DialConfig(c.cfg.URL, amqpConfig)
	if err != nil {
		return err
	}

	c.mu.Lock()

	oldConn := c.conn
	c.conn = conn

	c.notify = make(chan *amqp091.Error, 1)
	conn.NotifyClose(c.notify)

	c.mu.Unlock()

	if oldConn != nil {
		_ = oldConn.Close()
	}

	go c.watchConnection()

	return nil
}

func (c *RabbitClient) watchConnection() {
	select {
	case <-c.ctx.Done():
		return
	case err := <-c.notify:
		if err != nil && c.closed.Load() {
			go c.reconnectLoop()
		}
	}
}

func (c *RabbitClient) reconnectLoop() {
	delay := c.cfg.ReconnectStrategy.Delay
	maxAttempts := c.cfg.ReconnectStrategy.Attempts

	for attempt := 0; !c.closed.Load() && attempt <= maxAttempts; attempt++ {
		if err := c.connect(); err == nil {
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = time.Duration(float64(delay) * c.cfg.ReconnectStrategy.Backoff)
	}
}

func (c *RabbitClient) GetChannel() (*amqp091.Channel, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("rabbitmq client closed")
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("channel error")
	}

	return conn.Channel()
}

func (c *RabbitClient) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *RabbitClient) Context() context.Context {
	return c.ctx
}

func (c *RabbitClient) ExchangeDeclare(
	name, kind string,
	durable, autoDelete, internal, noWait bool,
	args amqp091.Table) error {

	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

	defer ch.Close()

	return ch.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (c *RabbitClient) DeclareAndBindQueue(
	name, routingKey, exchangeName string,
	durable, autoDelete, exclusive, noWait bool,
	args amqp091.Table) error {

	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

	defer ch.Close()

	_, err = ch.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
	if err != nil {
		return err
	}

	return ch.QueueBind(name, routingKey, exchangeName, noWait, args)
}
