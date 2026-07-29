package main

import (
	"NewsAggregator/internal/config"
	"NewsAggregator/internal/parser"
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

	service := services.New(rep, pool)

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
			service.SaveJobResult(ctx, jobRes)
		}
	}()

	wg.Wait()
}
