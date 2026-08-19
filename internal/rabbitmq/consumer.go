package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/rabbitmq/amqp091-go"
)

var ErrConsumerStopped = errors.New("consumer stopped")

type MessageHandler func(context.Context, amqp091.Delivery) error

type Consumer struct {
	client     *RabbitClient
	handler    MessageHandler
	done       chan struct{}
	queue      string
	workersNum int
}

func NewConsumer(c *RabbitClient, h MessageHandler, queue string) *Consumer {
	return &Consumer{
		client:     c,
		handler:    h,
		workersNum: c.cfg.ConsumerWorkersNum,
		queue:      queue,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.client.Context().Done():
			return c.client.Context().Err()
		default:
		}

		if err := c.consume(ctx); err != nil {
			continue
		}

		return nil
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	ch, err := c.client.GetChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer func() {
		if chErr := ch.Close(); err != nil {
			log.Printf("error closing channel %v", chErr)
		}
	}()

	msg, err := ch.Consume(
		c.queue,
		"",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < c.workersNum; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(workerCtx, msg)
		}()
	}

	go func() {
		wg.Wait()
		close(c.done)
	}()

	select {
	case <-ctx.Done():
		cancel()
		<-c.done
		return ctx.Err()

	case <-c.client.Context().Done():
		cancel()
		<-c.done
		return c.client.Context().Err()

	case <-c.done:
		return ErrConsumerStopped
	}
}

func (c *Consumer) worker(ctx context.Context, msgs <-chan amqp091.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			c.processDelivery(ctx, msg)
		}
	}
}

func (c *Consumer) processDelivery(ctx context.Context, msg amqp091.Delivery) {
	handleErr := c.handler(ctx, msg)

	if handleErr != nil {
		nackErr := msg.Nack(false, false)
		if nackErr != nil {
			log.Printf("failed to send NACK: %v", nackErr)
		}
	} else {
		ackErr := msg.Ack(false)
		if ackErr != nil {
			log.Printf("failed to send ACK: %v", ackErr)
		}
	}
}

func (c *Consumer) Setup() error {
	dlqArgs := amqp091.Table{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dlq_key",
	}

	err := c.client.ExchangeDeclare(
		"news",
		"direct",
		false,
		false,
		false,
		false,
		nil)

	if err != nil {
		return err
	}

	err = c.client.DeclareAndBindQueue(
		c.queue,
		"news",
		"news",
		false,
		false,
		false,
		false,
		dlqArgs)

	if err != nil {
		return err
	}

	err = c.client.ExchangeDeclare(
		"dlx",
		"direct",
		false,
		false,
		false,
		false,
		nil)

	if err != nil {
		return err
	}

	err = c.client.DeclareAndBindQueue(
		"dlq",
		"dlq_key",
		"dlx",
		false,
		false,
		false,
		false,
		nil)

	if err != nil {
		return err
	}

	return nil
}
