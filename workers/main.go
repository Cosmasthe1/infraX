package main

import (
	"flag"
	"log"
	"time"
)

func main() {
	interval := flag.Duration("interval", 5*time.Second, "work poll interval")
	flag.Parse()
	log.Println("worker starting (skeleton) — polling every", interval)
	for {
		// Placeholder: connect to NATS/Redis and process jobs
		log.Println("poll: looking for work (placeholder)")
		time.Sleep(*interval)
	}
}
