package main

import (
	"database/sql"
	"errors"
	"testing"
)

type fakeDB struct {
	query string
	args  []any
	err   error
}

func (f *fakeDB) Exec(query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	return nil, f.err
}

func (f *fakeDB) Query(query string, args ...any) (*sql.Rows, error) {
	return nil, f.err
}

func (f *fakeDB) QueryRow(query string, args ...any) *sql.Row {
	return nil
}

func TestRecordDeploymentUsesInjectedExecutor(t *testing.T) {
	fake := &fakeDB{}

	err := recordDeployment(fake, "tenant-a", "registry/acme:1.0")
	if err != nil {
		t.Fatalf("recordDeployment returned an unexpected error: %v", err)
	}

	if fake.query == "" {
		t.Fatal("expected Exec to be called")
	}
	if len(fake.args) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(fake.args))
	}
	if got := fake.args[0]; got != "tenant-a" {
		t.Fatalf("tenant argument mismatch: got %v", got)
	}
	if got := fake.args[1]; got != "registry/acme:1.0" {
		t.Fatalf("image argument mismatch: got %v", got)
	}
}

func TestRecordDeploymentPropagatesExecError(t *testing.T) {
	expected := errors.New("boom")
	fake := &fakeDB{err: expected}

	err := recordDeployment(fake, "tenant-b", "registry/acme:2.0")
	if !errors.Is(err, expected) {
		t.Fatalf("expected %v, got %v", expected, err)
	}
}

func TestUpdateDeploymentStatus(t *testing.T) {
	fake := &fakeDB{}
	if err := setDeploymentStatus(fake, 42, StatusRunning); err != nil {
		t.Fatalf("setDeploymentStatus returned unexpected error: %v", err)
	}
	if fake.query == "" {
		t.Fatal("expected Exec to be invoked for status update")
	}
}
