package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MatchCommunication fails closed unless destination is positively listed.
func MatchCommunication(authority json.RawMessage, destination string) error {
	return matchTarget(authority, "communication_authority", "destination", destination, ErrUnknownDestination)
}

// MatchExecution fails closed unless action is positively listed.
func MatchExecution(authority json.RawMessage, action string) error {
	return matchTarget(authority, "execution_authority", "action", action, ErrUnknownAction)
}

func matchTarget(authority json.RawMessage, field, key, value string, miss error) error {
	if strings.TrimSpace(value) == "" || forbiddenWildcard(value) {
		return fmt.Errorf("%w: %s", miss, value)
	}
	items, err := requireJSONArray(field, authority)
	if err != nil {
		return err
	}
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s items must be objects", ErrMalformedAuthority, field)
		}
		raw, ok := obj[key]
		if !ok {
			return fmt.Errorf("%w: %s item missing %s", ErrMalformedAuthority, field, key)
		}
		listed, ok := asString(raw)
		if !ok {
			return fmt.Errorf("%w: %s %s must be a string", ErrMalformedAuthority, field, key)
		}
		if forbiddenWildcard(listed) {
			return fmt.Errorf("%w: %s %s wildcard or empty value", ErrMalformedAuthority, field, key)
		}
		if listed == value {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", miss, value)
}

// ApprovalRequired reports whether the mandate still needs an approval grant.
// Phase 3 has no approval workflow, so any positive requirement is unmet.
func ApprovalRequired(raw json.RawMessage) (bool, error) {
	obj, err := requireJSONObject("approval_requirements", raw)
	if err != nil {
		return false, err
	}
	if len(obj) == 0 {
		return false, nil
	}
	if v, ok := obj["human"]; ok {
		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("%w: approval_requirements.human", ErrMalformedAuthority)
		}
		if b {
			return true, nil
		}
	}
	if v, ok := obj["required"]; ok {
		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("%w: approval_requirements.required", ErrMalformedAuthority)
		}
		if b {
			return true, nil
		}
	}
	for _, key := range []string{"count", "min_approvals"} {
		if v, ok := obj[key]; ok {
			n, err := parseDecimal(v)
			if err != nil {
				return false, fmt.Errorf("%w: approval_requirements.%s", err, key)
			}
			if n.Sign() > 0 {
				return true, nil
			}
		}
	}
	for _, key := range []string{"approvers", "requirements"} {
		if v, ok := obj[key]; ok {
			arr, ok := v.([]any)
			if !ok {
				return false, fmt.Errorf("%w: approval_requirements.%s must be an array", ErrMalformedAuthority, key)
			}
			if len(arr) > 0 {
				return true, nil
			}
		}
	}
	for key := range obj {
		if !approvalKnownKey(key) {
			return false, fmt.Errorf("%w: approval_requirements unknown key %s", ErrMalformedAuthority, key)
		}
	}
	return false, nil
}

// AssertBudgetAllowsSpend fails closed when the act exceeds remaining budget.
func AssertBudgetAllowsSpend(budgetRaw, act json.RawMessage) error {
	budget, err := parseBudget(budgetRaw)
	if err != nil {
		return err
	}
	if !budget.present || budget.remaining.Sign() <= 0 {
		return fmt.Errorf("%w: no remaining budget", ErrBudgetInsufficient)
	}
	obj, err := requireJSONObject("act", act)
	if err != nil {
		return err
	}
	rawAmount, ok := obj["amount"]
	if !ok {
		return fmt.Errorf("%w: act.amount", ErrRequiredField)
	}
	amount, err := parseDecimal(rawAmount)
	if err != nil {
		return fmt.Errorf("%w: act.amount", err)
	}
	if amount.Sign() <= 0 {
		return fmt.Errorf("%w: act.amount must be positive", ErrInvalidInput)
	}
	unit, _ := asString(obj["unit"])
	if strings.TrimSpace(unit) == "" {
		return fmt.Errorf("%w: act.unit", ErrRequiredField)
	}
	if unit != budget.unit {
		return fmt.Errorf("%w: unit", ErrBudgetInsufficient)
	}
	if amount.Cmp(budget.remaining) > 0 {
		return fmt.Errorf("%w: amount exceeds remaining", ErrBudgetInsufficient)
	}
	if rawCat, ok := obj["category"]; ok {
		cat, ok := asString(rawCat)
		if !ok || strings.TrimSpace(cat) == "" {
			return fmt.Errorf("%w: act.category", ErrMalformedAuthority)
		}
		if _, listed := stringSet(budget.categories)[cat]; !listed {
			return fmt.Errorf("%w: category", ErrBudgetInsufficient)
		}
	}
	return nil
}

// AssertConstraintsHold evaluates mandate constraints against the act and now.
func AssertConstraintsHold(constraints, act json.RawMessage, now time.Time) error {
	obj, err := requireJSONObject("constraints", constraints)
	if err != nil {
		return err
	}
	if len(obj) == 0 {
		return nil
	}
	actObj, err := requireJSONObject("act", act)
	if err != nil {
		return err
	}
	now = normalizeTime(now)
	if err := requireTime("now", now); err != nil {
		return err
	}
	for key, pv := range obj {
		switch {
		case isUpperBound(key):
			if err := constrainUpperBound(key, pv, actObj); err != nil {
				return err
			}
		case isUpperTime(key):
			limit, err := parseConstraintTime(pv)
			if err != nil {
				return fmt.Errorf("%w: constraints.%s", err, key)
			}
			if now.After(limit) {
				return fmt.Errorf("%w: constraints.%s", ErrConstraintViolated, key)
			}
		case isLowerTime(key):
			limit, err := parseConstraintTime(pv)
			if err != nil {
				return fmt.Errorf("%w: constraints.%s", err, key)
			}
			if now.Before(limit) {
				return fmt.Errorf("%w: constraints.%s", ErrConstraintViolated, key)
			}
		case isSetConstraint(key):
			if err := constrainSet(key, pv, actObj); err != nil {
				return err
			}
		default:
			if !knownConstraintKey(key) {
				return fmt.Errorf("%w: constraints unknown key %s", ErrMalformedAuthority, key)
			}
		}
	}
	return nil
}

func isUpperBound(key string) bool {
	_, ok := knownUpperBoundConstraints[key]
	return ok
}

func isUpperTime(key string) bool {
	_, ok := knownUpperTimeConstraints[key]
	return ok
}

func isLowerTime(key string) bool {
	_, ok := knownLowerTimeConstraints[key]
	return ok
}

func isSetConstraint(key string) bool {
	_, ok := knownSetConstraints[key]
	return ok
}

func constrainUpperBound(key string, pv any, act map[string]any) error {
	limit, err := parseDecimal(pv)
	if err != nil {
		return fmt.Errorf("%w: constraints.%s", err, key)
	}
	raw, ok := actValue(act, key, "amount", "rate", "max_amount")
	if !ok {
		return nil
	}
	n, err := parseDecimal(raw)
	if err != nil {
		return fmt.Errorf("%w: act.%s", err, key)
	}
	if n.Cmp(limit) > 0 {
		return fmt.Errorf("%w: constraints.%s", ErrConstraintViolated, key)
	}
	return nil
}

func constrainSet(key string, pv any, act map[string]any) error {
	raw, ok := act[key]
	if !ok {
		return fmt.Errorf("%w: act missing %s", ErrConstraintViolated, key)
	}
	okSubset, err := constraintSetSubset(raw, pv)
	if err != nil {
		return fmt.Errorf("%w: constraints.%s", err, key)
	}
	if !okSubset {
		return fmt.Errorf("%w: constraints.%s", ErrConstraintViolated, key)
	}
	return nil
}

func actValue(act map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if v, ok := act[key]; ok {
			return v, true
		}
	}
	return nil, false
}
