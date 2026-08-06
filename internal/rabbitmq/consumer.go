package rabbitmq

import (
	"context"
	"fmt"
	"github.com/rabbitmq/amqp091-go"
	"log"
	"sync"
)

type MessageHandler func(context.Context, amqp091.Delivery) error

type Consumer struct {
	client     *RabbitClient
	handler    MessageHandler
	workersNum int
	queue      string
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
			return fmt.Errorf("client context done")
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
	defer ch.Close()

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

	select {
	case <-ctx.Done():
		cancel()
		wg.Wait()
		return ctx.Err()
	case <-c.client.Context().Done():
		cancel()
		wg.Wait()
		return fmt.Errorf("client context done")
	default:
		wg.Wait()
		cancel()
		return fmt.Errorf("consumer workers stopped")
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
		nackErr := msg.Nack(false, true)
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
