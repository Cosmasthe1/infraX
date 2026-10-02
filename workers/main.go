package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	interval := flag.Duration("interval", 5*time.Second, "work poll interval")
	flag.Parse()

	queue := &MemoryQueue{}
	for _, job := range []string{"deploy:sample-app", "deploy:worker-service"} {
		queue.Enqueue(job)
	}

	poller := &Poller{
		Interval: *interval,
		Queue:    queue,
		Process: func(job string) error {
			log.Printf("processing job: %s", job)
			return nil
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("worker starting — polling every %s", interval)
	if err := poller.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("worker failed: %v", err)
	}
}
