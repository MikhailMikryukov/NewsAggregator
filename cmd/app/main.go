package main

import (
	"NewsAggregator/internal/config"
	"NewsAggregator/internal/parser"
	"NewsAggregator/internal/repository"
	"NewsAggregator/internal/services"
	"NewsAggregator/internal/workers"
	"context"
	"log"
	"time"
)

func main() {
	cfg := config.Load()

	rep, err := repository.NewRepository(cfg.DBConnectionString)
	if err != nil {
		log.Fatal(err.Error())
	}

	rssParser := parser.NewRSSParser(10 * time.Second)

	pool := workers.New(cfg.RssWorkersNum, rssParser)

	service := services.New(rep, pool)

	service.SetRssJobs()

	ctx := context.TODO()
	pool.Start(ctx)

	go func() {
		for jobRes := range pool.Results() {
			service.SaveJobResult(ctx, jobRes)
		}
	}()

}
