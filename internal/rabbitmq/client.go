package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/MikhailMikryukov/NewsAggregator/internal/config"
)

const (
	defaultConnectionTimeout = 10 * time.Second
	defaultHeartBeat         = 10 * time.Second
)

var (
	ErrEmptyURL       = errors.New("URL cannot be empty")
	ErrClosedClient   = errors.New("rabbitmq client closed")
	ErrGettingChannel = errors.New("getting channel error")
)

type RabbitClient struct {
	ctx    context.Context
	conn   *amqp091.Connection
	notify chan *amqp091.Error
	cancel context.CancelFunc
	cfg    config.RabbitConfig
	mu     sync.RWMutex
	closed atomic.Bool
}

func NewClient(cfg config.RabbitConfig) (*RabbitClient, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("%w", ErrEmptyURL)
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
		Dial:      dialer.Dial,
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
		if err != nil && !c.closed.Load() {
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
		return nil, fmt.Errorf("%w", ErrClosedClient)
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("%w", ErrGettingChannel)
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

	defer func() {
		if err := ch.Close(); err != nil {
			log.Printf("error closing channel %v", err)
		}
	}()

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

	defer func() {
		if chErr := ch.Close(); err != nil {
			log.Printf("error closing channel %v", chErr)
		}
	}()

	_, err = ch.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
	if err != nil {
		return err
	}

	return ch.QueueBind(name, routingKey, exchangeName, noWait, args)
}
