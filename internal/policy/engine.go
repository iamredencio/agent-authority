package policy

import (
	"context"
	"fmt"

	"github.com/iamredencio/agent-authority/internal/domain"
	"github.com/open-policy-agent/opa/v1/rego"
)

const (
	DefaultModulePath = "authority/decision.rego"
	DefaultQuery      = "data.authority.decision.allow"
)

// DefaultModule adds no extra restriction after mandate checks have passed.
const DefaultModule = `
package authority.decision

# Mandate, mission, identity and authority-set checks already ran in process.
# This default policy does not grant anything the mandate did not already grant.
default allow := true
`

// Engine evaluates OPA/Rego in-process (D-021).
type Engine struct {
	query rego.PreparedEvalQuery
}

func New(modulePath, module string) (*Engine, error) {
	if modulePath == "" {
		modulePath = DefaultModulePath
	}
	if module == "" {
		module = DefaultModule
	}
	prepared, err := rego.New(
		rego.Query(DefaultQuery),
		rego.Module(modulePath, module),
	).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("%w: compile: %v", domain.ErrPolicyError, err)
	}
	return &Engine{query: prepared}, nil
}

func Default() *Engine {
	engine, err := New(DefaultModulePath, DefaultModule)
	if err != nil {
		panic(err)
	}
	return engine
}

// Allow reports whether policy permits an already mandate-authorized act.
// A false result or any evaluation error is fail-closed deny.
func (e *Engine) Allow(ctx context.Context, input any) (bool, error) {
	if e == nil {
		return false, fmt.Errorf("%w: policy engine missing", domain.ErrPolicyError)
	}
	rs, err := e.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("%w: %v", domain.ErrPolicyError, err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return false, fmt.Errorf("%w: empty result", domain.ErrPolicyError)
	}
	allow, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		return false, fmt.Errorf("%w: allow is not boolean", domain.ErrPolicyError)
	}
	if !allow {
		return false, fmt.Errorf("%w", domain.ErrPolicyDenied)
	}
	return true, nil
}
