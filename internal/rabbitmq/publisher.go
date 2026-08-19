package rabbitmq

import (
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	client      *RabbitClient
	exchange    string
	contentType string
}

func NewPublisher(client *RabbitClient, exchange, contentType string) *Publisher {
	return &Publisher{
		client:      client,
		exchange:    exchange,
		contentType: contentType,
	}
}

func (p *Publisher) Publish(routingKey string, body []byte) error {
	ch, err := p.client.GetChannel()
	if err != nil {
		return err
	}
	defer func() {
		if chErr := ch.Close(); err != nil {
			log.Printf("error closing channel %v", chErr)
		}
	}()

	msg := amqp091.Publishing{
		ContentType: p.contentType,
		Body:        body,
	}

	maxAttempts := p.client.cfg.PublishingStrategy.Attempts
	delay := p.client.cfg.PublishingStrategy.Delay
	backoff := p.client.cfg.ReconnectStrategy.Backoff

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		err = ch.Publish(p.exchange, routingKey, true, false, msg)
		if err == nil {
			return nil
		}

		select {
		case <-p.client.ctx.Done():
			return p.client.ctx.Err()
		case <-time.After(delay):
		}
		delay = time.Duration(float64(delay) * backoff)
	}

	return err
}
