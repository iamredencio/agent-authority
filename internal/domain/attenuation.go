package domain

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// AssertAttenuation verifies specification §14.3: the child is no broader than
// the parent on every authority axis. Equality is allowed except on
// delegation_depth, which must strictly decrease.
func AssertAttenuation(child, parent Mandate, childMission, parentMission Mission, childMissionParent *Mission, now time.Time) error {
	if err := parent.Validate(); err != nil {
		return err
	}
	if err := child.Validate(); err != nil {
		return err
	}
	if err := SameOrganization(parent.OrganizationID, child.OrganizationID); err != nil {
		return err
	}
	if child.ParentMandateID == nil || *child.ParentMandateID != parent.MandateID {
		return fmt.Errorf("%w: parent_mandate_id", ErrInvalidReference)
	}

	if err := assertPermissionSubset("scope", child.Scope, parent.Scope, true); err != nil {
		return err
	}
	if err := assertTargetSubset("communication_authority", child.CommunicationAuthority, parent.CommunicationAuthority, "destination"); err != nil {
		return err
	}
	if err := assertTargetSubset("execution_authority", child.ExecutionAuthority, parent.ExecutionAuthority, "action"); err != nil {
		return err
	}
	if err := assertConstraintsTighterOrEqual(child.Constraints, parent.Constraints); err != nil {
		return err
	}
	if err := assertBudgetAttenuated(child.Budget, parent.Budget); err != nil {
		return err
	}
	if child.Expiry.After(parent.Expiry) {
		return fmt.Errorf("%w: expiry", ErrAmplification)
	}
	if child.NotBefore.Before(parent.NotBefore) {
		return fmt.Errorf("%w: not_before", ErrAmplification)
	}
	if err := assertDelegationDepth(child, parent); err != nil {
		return err
	}
	if err := assertApprovalStricterOrEqual(child.ApprovalRequirements, parent.ApprovalRequirements); err != nil {
		return err
	}
	if err := assertEvidenceStricterOrEqual(child.EvidenceRequirements, parent.EvidenceRequirements); err != nil {
		return err
	}
	if err := assertMissionAxis(child, parent, childMission, parentMission, childMissionParent, now); err != nil {
		return err
	}
	if child.AuthoritySourceID != parent.AuthoritySourceID {
		// No source-widening lattice is defined. A different source cannot be
		// proven not to widen the grant, so it fails closed.
		return fmt.Errorf("%w: authority_source", ErrAmplification)
	}
	return nil
}

func assertDelegationDepth(child, parent Mandate) error {
	if parent.DelegationDepth <= 0 {
		return fmt.Errorf("%w: parent delegation_depth is 0", ErrCannotDelegate)
	}
	if child.DelegationDepth < 0 {
		return fmt.Errorf("%w: delegation_depth", ErrInvalidInput)
	}
	if child.DelegationDepth >= parent.DelegationDepth {
		return fmt.Errorf("%w: delegation_depth must strictly decrease", ErrAmplification)
	}
	return nil
}

func assertPermissionSubset(field string, childRaw, parentRaw json.RawMessage, allowObject bool) error {
	child, err := decodeJSON(childRaw)
	if err != nil {
		return fmt.Errorf("%w: %s", err, field)
	}
	parent, err := decodeJSON(parentRaw)
	if err != nil {
		return fmt.Errorf("%w: %s", err, field)
	}
	ok, err := permissionSubset(child, parent, allowObject)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrMalformedAuthority, field, err)
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrAmplification, field)
	}
	return nil
}

func permissionSubset(child, parent any, allowObject bool) (bool, error) {
	switch p := parent.(type) {
	case map[string]any:
		if !allowObject {
			return false, fmt.Errorf("object not allowed")
		}
		c, ok := child.(map[string]any)
		if !ok {
			return false, fmt.Errorf("type mismatch")
		}
		for key, cv := range c {
			pv, exists := p[key]
			if !exists {
				return false, nil
			}
			nested, err := permissionSubset(cv, pv, true)
			if err != nil {
				return false, err
			}
			if !nested {
				return false, nil
			}
		}
		return true, nil
	case []any:
		c, ok := child.([]any)
		if !ok {
			return false, fmt.Errorf("type mismatch")
		}
		parentSet := make(map[string]struct{}, len(p))
		for _, item := range p {
			canon, err := canonicalValue(item)
			if err != nil {
				return false, err
			}
			parentSet[canon] = struct{}{}
		}
		for _, item := range c {
			canon, err := canonicalValue(item)
			if err != nil {
				return false, err
			}
			if _, ok := parentSet[canon]; !ok {
				return false, nil
			}
		}
		return true, nil
	default:
		return valuesEqual(child, parent)
	}
}

func assertTargetSubset(field string, childRaw, parentRaw json.RawMessage, requiredKey string) error {
	child, err := requireJSONArray(field, childRaw)
	if err != nil {
		return err
	}
	parent, err := requireJSONArray(field, parentRaw)
	if err != nil {
		return err
	}
	parentSet := make(map[string]struct{}, len(parent))
	for _, item := range parent {
		canon, err := canonicalTarget(field, item, requiredKey)
		if err != nil {
			return err
		}
		parentSet[canon] = struct{}{}
	}
	for _, item := range child {
		canon, err := canonicalTarget(field, item, requiredKey)
		if err != nil {
			return err
		}
		if _, ok := parentSet[canon]; !ok {
			return fmt.Errorf("%w: %s", ErrAmplification, field)
		}
	}
	return nil
}

func canonicalTarget(field string, item any, requiredKey string) (string, error) {
	obj, ok := item.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%w: %s items must be objects", ErrMalformedAuthority, field)
	}
	raw, ok := obj[requiredKey]
	if !ok {
		return "", fmt.Errorf("%w: %s item missing %s", ErrMalformedAuthority, field, requiredKey)
	}
	value, ok := asString(raw)
	if !ok {
		return "", fmt.Errorf("%w: %s %s must be a string", ErrMalformedAuthority, field, requiredKey)
	}
	if forbiddenWildcard(value) {
		return "", fmt.Errorf("%w: %s %s wildcard or empty value", ErrMalformedAuthority, field, requiredKey)
	}
	return canonicalValue(obj)
}

var knownUpperBoundConstraints = map[string]struct{}{
	"rate": {}, "max_rate": {}, "max_amount": {}, "limit": {},
}
var knownUpperTimeConstraints = map[string]struct{}{
	"expiry": {}, "until": {},
}
var knownLowerTimeConstraints = map[string]struct{}{
	"not_before": {}, "after": {},
}
var knownSetConstraints = map[string]struct{}{
	"geo": {}, "data": {}, "residency": {},
}

func assertConstraintsTighterOrEqual(childRaw, parentRaw json.RawMessage) error {
	child, err := requireJSONObject("constraints", childRaw)
	if err != nil {
		return err
	}
	parent, err := requireJSONObject("constraints", parentRaw)
	if err != nil {
		return err
	}
	for key, pv := range parent {
		cv, ok := child[key]
		if !ok {
			return fmt.Errorf("%w: constraints removed %s", ErrAmplification, key)
		}
		if err := constraintValueTighterOrEqual(key, cv, pv); err != nil {
			return err
		}
	}
	for key, cv := range child {
		if _, exists := parent[key]; exists {
			continue
		}
		if !knownConstraintKey(key) {
			return fmt.Errorf("%w: constraints added unknown key %s", ErrMalformedAuthority, key)
		}
		if err := constraintValueTighterOrEqual(key, cv, nil); err != nil {
			return err
		}
	}
	return nil
}

func knownConstraintKey(key string) bool {
	if _, ok := knownUpperBoundConstraints[key]; ok {
		return true
	}
	if _, ok := knownUpperTimeConstraints[key]; ok {
		return true
	}
	if _, ok := knownLowerTimeConstraints[key]; ok {
		return true
	}
	if _, ok := knownSetConstraints[key]; ok {
		return true
	}
	return false
}

func constraintValueTighterOrEqual(key string, child, parent any) error {
	if parent == nil {
		return nil
	}
	if _, ok := knownUpperBoundConstraints[key]; ok {
		c, err := parseDecimal(child)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		p, err := parseDecimal(parent)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		if c.Cmp(p) > 0 {
			return fmt.Errorf("%w: constraints.%s", ErrAmplification, key)
		}
		return nil
	}
	if _, ok := knownUpperTimeConstraints[key]; ok {
		c, err := parseConstraintTime(child)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		p, err := parseConstraintTime(parent)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		if c.After(p) {
			return fmt.Errorf("%w: constraints.%s", ErrAmplification, key)
		}
		return nil
	}
	if _, ok := knownLowerTimeConstraints[key]; ok {
		c, err := parseConstraintTime(child)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		p, err := parseConstraintTime(parent)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		if c.Before(p) {
			return fmt.Errorf("%w: constraints.%s", ErrAmplification, key)
		}
		return nil
	}
	if _, ok := knownSetConstraints[key]; ok {
		okSubset, err := constraintSetSubset(child, parent)
		if err != nil {
			return fmt.Errorf("%w: constraints.%s", err, key)
		}
		if !okSubset {
			return fmt.Errorf("%w: constraints.%s", ErrAmplification, key)
		}
		return nil
	}
	equal, err := valuesEqual(child, parent)
	if err != nil {
		return fmt.Errorf("%w: constraints.%s", err, key)
	}
	if !equal {
		return fmt.Errorf("%w: constraints.%s", ErrAmplification, key)
	}
	return nil
}

func parseConstraintTime(v any) (time.Time, error) {
	s, ok := asString(v)
	if !ok {
		return time.Time{}, fmt.Errorf("%w: time constraint must be an RFC3339 string", ErrMalformedAuthority)
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrMalformedAuthority, err)
	}
	return t.UTC(), nil
}

func constraintSetSubset(child, parent any) (bool, error) {
	if cs, ok := asString(child); ok {
		if ps, ok := asString(parent); ok {
			return cs == ps, nil
		}
		if parr, ok := asStringSlice(parent); ok {
			_, exists := stringSet(parr)[cs]
			return exists, nil
		}
		return false, fmt.Errorf("%w: type mismatch", ErrMalformedAuthority)
	}
	carr, ok := asStringSlice(child)
	if !ok {
		return false, fmt.Errorf("%w: must be a string or string array", ErrMalformedAuthority)
	}
	if ps, ok := asString(parent); ok {
		if len(carr) == 0 {
			return true, nil
		}
		return len(carr) == 1 && carr[0] == ps, nil
	}
	parr, ok := asStringSlice(parent)
	if !ok {
		return false, fmt.Errorf("%w: type mismatch", ErrMalformedAuthority)
	}
	pset := stringSet(parr)
	for _, item := range carr {
		if _, ok := pset[item]; !ok {
			return false, nil
		}
	}
	return true, nil
}

type budgetDoc struct {
	present    bool
	unit       string
	amount     *big.Rat
	remaining  *big.Rat
	categories []string
}

func parseBudget(raw json.RawMessage) (budgetDoc, error) {
	obj, err := requireJSONObject("budget", raw)
	if err != nil {
		return budgetDoc{}, err
	}
	if len(obj) == 0 {
		return budgetDoc{}, nil
	}
	required := []string{"unit", "amount", "remaining", "categories"}
	for _, key := range required {
		if _, ok := obj[key]; !ok {
			return budgetDoc{}, fmt.Errorf("%w: budget missing %s", ErrMalformedAuthority, key)
		}
	}
	for key := range obj {
		switch key {
		case "unit", "amount", "remaining", "categories":
		default:
			return budgetDoc{}, fmt.Errorf("%w: budget unknown field %s", ErrMalformedAuthority, key)
		}
	}
	unit, ok := asString(obj["unit"])
	if !ok || strings.TrimSpace(unit) == "" {
		return budgetDoc{}, fmt.Errorf("%w: budget.unit", ErrMalformedAuthority)
	}
	amount, err := parseDecimal(obj["amount"])
	if err != nil {
		return budgetDoc{}, fmt.Errorf("%w: budget.amount", err)
	}
	remaining, err := parseDecimal(obj["remaining"])
	if err != nil {
		return budgetDoc{}, fmt.Errorf("%w: budget.remaining", err)
	}
	if remaining.Cmp(amount) > 0 {
		return budgetDoc{}, fmt.Errorf("%w: budget.remaining exceeds amount", ErrMalformedAuthority)
	}
	if amount.Sign() < 0 || remaining.Sign() < 0 {
		return budgetDoc{}, fmt.Errorf("%w: budget amounts cannot be negative", ErrMalformedAuthority)
	}
	cats, ok := asStringSlice(obj["categories"])
	if !ok {
		return budgetDoc{}, fmt.Errorf("%w: budget.categories must be a string array", ErrMalformedAuthority)
	}
	return budgetDoc{
		present:    true,
		unit:       unit,
		amount:     amount,
		remaining:  remaining,
		categories: cats,
	}, nil
}

func assertBudgetAttenuated(childRaw, parentRaw json.RawMessage) error {
	child, err := parseBudget(childRaw)
	if err != nil {
		return err
	}
	parent, err := parseBudget(parentRaw)
	if err != nil {
		return err
	}
	if !parent.present {
		if child.present {
			return fmt.Errorf("%w: budget", ErrAmplification)
		}
		return nil
	}
	if !child.present {
		return nil
	}
	if child.unit != parent.unit {
		return fmt.Errorf("%w: budget.unit", ErrAmplification)
	}
	if child.amount.Cmp(parent.remaining) > 0 || child.remaining.Cmp(parent.remaining) > 0 {
		return fmt.Errorf("%w: budget exceeds remaining parent budget", ErrAmplification)
	}
	pset := stringSet(parent.categories)
	for _, cat := range child.categories {
		if _, ok := pset[cat]; !ok {
			return fmt.Errorf("%w: budget.categories", ErrAmplification)
		}
	}
	return nil
}

func assertApprovalStricterOrEqual(childRaw, parentRaw json.RawMessage) error {
	child, err := requireJSONObject("approval_requirements", childRaw)
	if err != nil {
		return err
	}
	parent, err := requireJSONObject("approval_requirements", parentRaw)
	if err != nil {
		return err
	}
	return stricterOrEqualObject("approval_requirements", child, parent, approvalKnownKey)
}

func approvalKnownKey(key string) bool {
	switch key {
	case "human", "required", "count", "min_approvals", "approvers", "requirements":
		return true
	default:
		return false
	}
}

func evidenceKnownKey(key string) bool {
	switch key {
	case "types", "required", "records":
		return true
	default:
		return false
	}
}

func assertEvidenceStricterOrEqual(childRaw, parentRaw json.RawMessage) error {
	childVal, err := decodeJSON(childRaw)
	if err != nil {
		return fmt.Errorf("%w: evidence_requirements", err)
	}
	parentVal, err := decodeJSON(parentRaw)
	if err != nil {
		return fmt.Errorf("%w: evidence_requirements", err)
	}
	switch parent := parentVal.(type) {
	case []any:
		child, ok := childVal.([]any)
		if !ok {
			return fmt.Errorf("%w: evidence_requirements type mismatch", ErrMalformedAuthority)
		}
		ok, err := stringArraySuperset(child, parent)
		if err != nil {
			return fmt.Errorf("%w: evidence_requirements", err)
		}
		if !ok {
			return fmt.Errorf("%w: evidence_requirements", ErrAmplification)
		}
		return nil
	case map[string]any:
		child, ok := childVal.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: evidence_requirements type mismatch", ErrMalformedAuthority)
		}
		return stricterOrEqualObject("evidence_requirements", child, parent, evidenceKnownKey)
	default:
		return fmt.Errorf("%w: evidence_requirements must be an array or object", ErrMalformedAuthority)
	}
}

func stringArraySuperset(child, parent []any) (bool, error) {
	cset := make(map[string]struct{}, len(child))
	for _, item := range child {
		s, ok := item.(string)
		if !ok {
			return false, fmt.Errorf("%w: array items must be strings", ErrMalformedAuthority)
		}
		cset[s] = struct{}{}
	}
	for _, item := range parent {
		s, ok := item.(string)
		if !ok {
			return false, fmt.Errorf("%w: array items must be strings", ErrMalformedAuthority)
		}
		if _, ok := cset[s]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func stricterOrEqualObject(field string, child, parent map[string]any, known func(string) bool) error {
	for key, pv := range parent {
		cv, ok := child[key]
		if !ok {
			if known(key) {
				if looserThanAbsent(key, pv) {
					continue
				}
			}
			return fmt.Errorf("%w: %s removed %s", ErrAmplification, field, key)
		}
		if err := stricterOrEqualValue(field, key, cv, pv, known); err != nil {
			return err
		}
	}
	for key, cv := range child {
		if _, exists := parent[key]; exists {
			continue
		}
		if !known(key) {
			return fmt.Errorf("%w: %s added unknown key %s", ErrMalformedAuthority, field, key)
		}
		if err := stricterOrEqualValue(field, key, cv, absentDefault(key), known); err != nil {
			return err
		}
	}
	return nil
}

func looserThanAbsent(key string, v any) bool {
	switch key {
	case "human", "required":
		b, ok := v.(bool)
		return ok && !b
	case "count", "min_approvals":
		n, err := parseDecimal(v)
		return err == nil && n.Sign() == 0
	default:
		return false
	}
}

func absentDefault(key string) any {
	switch key {
	case "human", "required":
		return false
	case "count", "min_approvals":
		return json.Number("0")
	case "approvers", "requirements", "types", "records":
		return []any{}
	default:
		return nil
	}
}

func stricterOrEqualValue(field, key string, child, parent any, known func(string) bool) error {
	switch key {
	case "human", "required":
		c, cok := child.(bool)
		p, pok := parent.(bool)
		if !cok || !pok {
			return fmt.Errorf("%w: %s.%s must be a boolean", ErrMalformedAuthority, field, key)
		}
		if p && !c {
			return fmt.Errorf("%w: %s.%s", ErrAmplification, field, key)
		}
		return nil
	case "count", "min_approvals":
		c, err := parseDecimal(child)
		if err != nil {
			return fmt.Errorf("%w: %s.%s", err, field, key)
		}
		p, err := parseDecimal(parent)
		if err != nil {
			return fmt.Errorf("%w: %s.%s", err, field, key)
		}
		if c.Cmp(p) < 0 {
			return fmt.Errorf("%w: %s.%s", ErrAmplification, field, key)
		}
		return nil
	case "approvers", "requirements", "types", "records":
		c, ok := child.([]any)
		if !ok {
			return fmt.Errorf("%w: %s.%s must be an array", ErrMalformedAuthority, field, key)
		}
		p, ok := parent.([]any)
		if !ok {
			return fmt.Errorf("%w: %s.%s must be an array", ErrMalformedAuthority, field, key)
		}
		ok, err := stringArraySuperset(c, p)
		if err != nil {
			return fmt.Errorf("%w: %s.%s", err, field, key)
		}
		if !ok {
			return fmt.Errorf("%w: %s.%s", ErrAmplification, field, key)
		}
		return nil
	default:
		if !known(key) {
			equal, err := valuesEqual(child, parent)
			if err != nil {
				return err
			}
			if !equal {
				return fmt.Errorf("%w: %s.%s", ErrAmplification, field, key)
			}
		}
		return nil
	}
}

func assertMissionAxis(child, parent Mandate, childMission, parentMission Mission, childMissionParent *Mission, now time.Time) error {
	if err := parentMission.Validate(); err != nil {
		return err
	}
	if err := childMission.Validate(); err != nil {
		return err
	}
	if parent.MissionID != parentMission.MissionID {
		return fmt.Errorf("%w: parent mission", ErrInvalidReference)
	}
	if child.MissionID != childMission.MissionID {
		return fmt.Errorf("%w: child mission", ErrInvalidReference)
	}
	if err := SameOrganization(parent.OrganizationID, parentMission.OrganizationID, childMission.OrganizationID); err != nil {
		return err
	}
	if err := childMission.AssertAuthorizes(now); err != nil {
		return err
	}
	if child.MissionID == parent.MissionID {
		return nil
	}
	if childMission.ParentMissionID == nil || *childMission.ParentMissionID != parent.MissionID {
		return fmt.Errorf("%w: mission is not the parent mission or an approved sub-mission", ErrAmplification)
	}
	if childMissionParent == nil {
		return fmt.Errorf("%w: parent mission of sub-mission cannot be proven", ErrBrokenChain)
	}
	if childMissionParent.MissionID != parent.MissionID {
		return fmt.Errorf("%w: sub-mission parent", ErrInvalidReference)
	}
	if err := childMissionParent.AssertAuthorizes(now); err != nil {
		return fmt.Errorf("%w: parent mission", err)
	}
	if err := childMission.validateWindowInside(*childMissionParent, "sub-mission"); err != nil {
		return err
	}
	return nil
}
