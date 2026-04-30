package version

import "testing"

func TestMatchRulesEmpty(t *testing.T) {
	if !MatchRules(nil, nil) {
		t.Fatal("nil rules should match")
	}
}

func TestMatchRulesAllow(t *testing.T) {
	rules := []Rule{
		{Action: "allow"},
		{Action: "disallow", OS: &OSRule{Name: "windows"}},
	}
	got := MatchRules(rules, nil)
	if HostOS() == "windows" && got {
		t.Errorf("expected disallow on windows")
	}
	if HostOS() != "windows" && !got {
		t.Errorf("expected allow on non-windows")
	}
}

func TestMatchRulesFeatureGate(t *testing.T) {
	rules := []Rule{{Action: "allow", Features: map[string]bool{"is_demo_user": true}}}
	if MatchRules(rules, nil) {
		t.Error("rule with required feature should not apply when feature missing")
	}
	if !MatchRules(rules, map[string]bool{"is_demo_user": true}) {
		t.Error("rule should apply when feature present")
	}
}
