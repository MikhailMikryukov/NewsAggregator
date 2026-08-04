package main

import (
	"NewsAggregator/internal/config"
	"NewsAggregator/internal/parser"
	"NewsAggregator/internal/rabbitmq"
	"NewsAggregator/internal/repository"
	"NewsAggregator/internal/services"
	"NewsAggregator/internal/workers"
	"context"
	"log"
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

	err = rabbitClient.DeclareAndBindQueue(
		"news-waiting",
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

	service := services.New(rep, pool, publisher)

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
