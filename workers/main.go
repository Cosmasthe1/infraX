package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func reportStatus(baseURL string, job DeploymentJob, status string, errMsg string) error {
	url := fmt.Sprintf("%s/deployments/%d", strings.TrimRight(baseURL, "/"), job.DeploymentID)
	body := map[string]any{
		"status":      status,
		"attempts":    job.Attempts,
		"max_retries": job.MaxRetries,
	}
	if errMsg != "" {
		body["last_error"] = errMsg
	}
	payload, _ := json.Marshal(body)
	req, rerr := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	if rerr != nil {
		return rerr
	}
	req.Header.Set("Content-Type", "application/json")
	if key := os.Getenv("ADMIN_API_KEY"); key != "" {
		req.Header.Set("X-API-Key", key)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status update failed: %s", resp.Status)
	}
	return nil
}

func main() {
	interval := flag.Duration("interval", 5*time.Second, "work poll interval")
	controlPlaneURL := flag.String("control-plane-url", "http://localhost:9090", "control plane base URL")
	flag.Parse()

	var queue Queue
	var broker Broker
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		var err error
		broker, err = NewNATSBroker(natsURL)
		if err != nil {
			log.Printf("nats broker unavailable, falling back to HTTP: %v", err)
		} else {
			queue = &BrokerQueue{Broker: broker}
		}
	}
	if queue == nil {
		queue = &HTTPQueue{BaseURL: *controlPlaneURL, Client: &http.Client{Timeout: 5 * time.Second}}
	}
	defer func() {
		if broker != nil {
			_ = broker.Close()
		}
	}()

	poller := &Poller{
		Interval: *interval,
		Queue:    queue,
		Process: func(jobString string) error {
			var job DeploymentJob
			if err := json.Unmarshal([]byte(jobString), &job); err != nil {
				return err
			}
			if job.DeploymentID == 0 {
				return fmt.Errorf("invalid deployment id in job: %s", jobString)
			}
			if err := reportStatus(*controlPlaneURL, job, "running", ""); err != nil {
				return err
			}
			job.Attempts++
			workErr := processDeploymentJob(job, func(image string) error {
				namespace := strings.TrimSpace(job.Namespace)
				if namespace == "" {
					namespace = strings.TrimSpace(job.Tenant)
				}
				if namespace == "" {
					namespace = "default"
				}
				log.Printf("processing deployment %d for tenant %s namespace %s environment %s image %s", job.DeploymentID, job.Tenant, namespace, job.Environment, image)
				return deployToKubernetesWithConfig(image, namespace, job.Environment, job.Config)
			})
			if workErr == nil {
				job.Attempts = 0
				return reportStatus(*controlPlaneURL, job, "succeeded", "")
			}
			if job.Attempts >= job.MaxRetries {
				return reportStatus(*controlPlaneURL, job, "failed", workErr.Error())
			}
			if retryErr := reportStatus(*controlPlaneURL, job, "queued", workErr.Error()); retryErr != nil {
				return retryErr
			}
			return nil
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("worker starting — polling every %s from %s", interval, *controlPlaneURL)
	if err := poller.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("worker failed: %v", err)
	}
}
