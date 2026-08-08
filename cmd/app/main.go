package main

import (
	"NewsAggregator/internal/ai"
	"NewsAggregator/internal/config"
	"NewsAggregator/internal/parser"
	"NewsAggregator/internal/rabbitmq"
	"NewsAggregator/internal/repository"
	"NewsAggregator/internal/services"
	"NewsAggregator/internal/workers"
	"context"
	"github.com/rabbitmq/amqp091-go"
	"log"
	"strconv"
	"sync"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.TODO()

	rep, err := repository.NewRepository(ctx, cfg.DBConnectionString)
	if err != nil {
		log.Fatal(err.Error())
	}

	rssParser := parser.NewRSSParser(10 * time.Second)

	pool := workers.New(cfg.RssWorkersNum, rssParser)

	aiClient := ai.NewOpenAIClient(cfg.AIConfig)

	rabbitClient, err := rabbitmq.NewClient(cfg.RabbitCfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rabbitClient.Close()

	publisher := rabbitmq.NewPublisher(rabbitClient, "news", "application/json")

	err = rabbitClient.ExchangeDeclare(
		"news",
		"direct",
		false,
		false,
		false,
		false,
		nil)

	if err != nil {
		log.Fatal(err.Error())
	}

	queueName := "news-waiting"
	err = rabbitClient.DeclareAndBindQueue(
		queueName,
		"news",
		"news",
		false,
		false,
		false,
		false,
		nil)

	if err != nil {
		log.Fatal(err.Error())
	}

	service := services.New(rep, pool, publisher, aiClient)

	handler := func(ctx context.Context, d amqp091.Delivery) error {
		msg := d.Body

		id, castErr := strconv.ParseInt(string(msg), 10, 64)
		if castErr != nil {
			log.Println(err.Error())
			return err
		}

		err = service.HandleArticle(ctx, id)
		return err
	}

	consumer := rabbitmq.NewConsumer(rabbitClient, handler, queueName)

	err = consumer.Start(ctx)
	if err != nil {
		log.Fatal(err.Error())
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		service.SetRssJobs(ctx)
	}()

	pool.Start(ctx)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for jobRes := range pool.Results() {
			service.HandleJobResult(ctx, jobRes)
		}
	}()

	wg.Wait()
}
