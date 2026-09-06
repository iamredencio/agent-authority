package postgres

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	dockerOnce        sync.Once
	dockerDSN         string
	dockerErr         error
	dockerContainerID string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if dockerContainerID != "" {
		_ = exec.Command("docker", "rm", "-f", dockerContainerID).Run()
	}
	os.Exit(code)
}

func testDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	dockerOnce.Do(func() {
		dockerDSN, dockerErr = startDockerPostgres()
	})
	if dockerErr != nil {
		t.Logf("docker postgres: %v", dockerErr)
		return ""
	}
	return dockerDSN
}

func startDockerPostgres() (string, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", err
	}
	run := exec.Command("docker", "run", "-d", "--rm",
		"-e", "POSTGRES_USER=aa",
		"-e", "POSTGRES_PASSWORD=aa",
		"-e", "POSTGRES_DB=agent_authority",
		"-p", "127.0.0.1::5432",
		"postgres:16-alpine",
	)
	out, err := run.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run: %w: %s", err, bytes.TrimSpace(out))
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", fmt.Errorf("docker run produced no container id")
	}
	dockerContainerID = id

	portOut, err := exec.Command("docker", "port", id, "5432/tcp").CombinedOutput()
	if err != nil {
		_ = exec.Command("docker", "rm", "-f", id).Run()
		return "", fmt.Errorf("docker port: %w: %s", err, bytes.TrimSpace(portOut))
	}
	hostPort := strings.TrimSpace(string(portOut))
	_, port, ok := strings.Cut(hostPort, ":")
	if !ok {
		_ = exec.Command("docker", "rm", "-f", id).Run()
		return "", fmt.Errorf("unexpected docker port output %q", hostPort)
	}
	dsn := fmt.Sprintf("postgres://aa:aa@127.0.0.1:%s/agent_authority?sslmode=disable", port)

	deadline := time.Now().Add(45 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		store, err := New(ctx, dsn)
		if err == nil {
			store.Close()
			cancel()
			return dsn, nil
		}
		cancel()
		last = err
		time.Sleep(500 * time.Millisecond)
	}
	_ = exec.Command("docker", "rm", "-f", id).Run()
	return "", fmt.Errorf("postgres did not become ready: %w", last)
}
