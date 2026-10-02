package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// Broker is the external message layer used by the worker to consume deploy jobs.
type Broker interface {
	Next(context.Context) (DeploymentJob, bool, error)
	Close() error
}

type NATSBroker struct {
	conn *nats.Conn
	sub  *nats.Subscription
}

type BrokerQueue struct {
	Broker Broker
}

func (q *BrokerQueue) Next() (string, bool, error) {
	if q == nil || q.Broker == nil {
		return "", false, nil
	}
	job, ok, err := q.Broker.Next(context.Background())
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return "", false, err
	}
	return string(payload), true, nil
}

func NewNATSBroker(url string) (*NATSBroker, error) {
	if url == "" {
		return nil, errors.New("nats url is empty")
	}
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	sub, err := conn.SubscribeSync("infrax.deployments")
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &NATSBroker{conn: conn, sub: sub}, nil
}

func (b *NATSBroker) Next(ctx context.Context) (DeploymentJob, bool, error) {
	if b == nil || b.sub == nil {
		return DeploymentJob{}, false, nil
	}
	if err := ctx.Err(); err != nil {
		return DeploymentJob{}, false, err
	}
	msg, err := b.sub.NextMsg(2 * time.Second)
	if errors.Is(err, nats.ErrTimeout) {
		return DeploymentJob{}, false, nil
	}
	if err != nil {
		return DeploymentJob{}, false, err
	}
	var job DeploymentJob
	if err := json.Unmarshal(msg.Data, &job); err != nil {
		return DeploymentJob{}, false, err
	}
	return job, true, nil
}

func (b *NATSBroker) Close() error {
	if b == nil {
		return nil
	}
	if b.sub != nil {
		_ = b.sub.Unsubscribe()
	}
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}

func (q *HTTPQueue) Publish(job DeploymentJob) error {
	if q == nil || q.BaseURL == "" {
		return fmt.Errorf("queue not configured")
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	url := strings.TrimRight(q.BaseURL, "/") + "/jobs"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("publish failed: %s", resp.Status)
	}
	return nil
}
