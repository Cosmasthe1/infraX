package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

// PublishDeploymentJob emits a deployment job to the configured NATS subject.
func PublishDeploymentJob(natsURL string, job DeploymentJob) error {
	if natsURL == "" {
		return nil
	}
	if job.DeploymentID == 0 {
		return fmt.Errorf("deployment id is required")
	}
	conn, err := nats.Connect(natsURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return conn.Publish("infrax.deployments", payload)
}

func BrokerURL() string {
	if url := os.Getenv("NATS_URL"); url != "" {
		return url
	}
	return ""
}

func NATSQueueURL() string {
	return BrokerURL()
}

func RetryDelay() time.Duration {
	return 5 * time.Second
}
