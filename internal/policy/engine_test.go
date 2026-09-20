package policy

import (
	"context"
	"errors"
	"testing"

	"github.com/iamredencio/agent-authority/internal/domain"
)

func TestDefaultPolicyAllowsAfterMandate(t *testing.T) {
	ok, err := Default().Allow(context.Background(), map[string]any{"act_type": "execute"})
	if err != nil || !ok {
		t.Fatalf("allow=%v err=%v", ok, err)
	}
}

func TestPolicyCanDeny(t *testing.T) {
	engine, err := New("deny.rego", `
package authority.decision
default allow := false
allow if { input.act.action == "never" }
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Allow(context.Background(), map[string]any{"act": map[string]any{"action": "draft_report"}})
	if !errors.Is(err, domain.ErrPolicyDenied) {
		t.Fatalf("err = %v", err)
	}
}

func TestPolicyCompileError(t *testing.T) {
	_, err := New("bad.rego", "not rego")
	if !errors.Is(err, domain.ErrPolicyError) {
		t.Fatalf("err = %v", err)
	}
}

func TestMissingEngineFailsClosed(t *testing.T) {
	var engine *Engine
	_, err := engine.Allow(context.Background(), map[string]any{})
	if !errors.Is(err, domain.ErrPolicyError) {
		t.Fatalf("err = %v", err)
	}
}
