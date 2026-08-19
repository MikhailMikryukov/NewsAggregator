package main

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/MikhailMikryukov/NewsAggregator/internal/ai"
	"github.com/MikhailMikryukov/NewsAggregator/internal/config"
	"github.com/MikhailMikryukov/NewsAggregator/internal/parser"
	"github.com/MikhailMikryukov/NewsAggregator/internal/rabbitmq"
	"github.com/MikhailMikryukov/NewsAggregator/internal/repository"
	"github.com/MikhailMikryukov/NewsAggregator/internal/services"
	"github.com/MikhailMikryukov/NewsAggregator/internal/workers"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.TODO()

	rep, err := repository.NewRepository(ctx, cfg.DBConnectionString)
	if err != nil {
		return err
	}

	rssParser := parser.NewRSSParser(10 * time.Second)

	pool := workers.New(cfg.RssWorkersNum, rssParser)

	aiClient := ai.NewOpenAIClient(cfg.AIConfig)

	rabbitClient, err := rabbitmq.NewClient(cfg.RabbitCfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := rabbitClient.Close(); err != nil {
			log.Println(err)
		}
	}()

	publisher := rabbitmq.NewPublisher(rabbitClient, "news", "application/json")

	service := services.New(rep, rep, pool, publisher, aiClient)

	handler := func(ctx context.Context, d amqp091.Delivery) error {
		msg := d.Body

		id, castErr := strconv.ParseInt(string(msg), 10, 64)
		if castErr != nil {
			log.Println(castErr.Error())
			return castErr
		}

		err = service.HandleArticle(ctx, id)
		return err
	}

	consumer := rabbitmq.NewConsumer(rabbitClient, handler, "news-waiting")

	err = consumer.Setup()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err = consumer.Start(ctx)
		if err != nil {
			log.Println(err.Error())
		}
	}()

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

	return nil
}
