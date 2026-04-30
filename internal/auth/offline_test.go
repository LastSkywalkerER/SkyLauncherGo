package auth

import "testing"

func TestLoginOfflineDeterministic(t *testing.T) {
	a, err := LoginOffline("Alice")
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoginOffline("Alice")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Errorf("offline UUID not deterministic: %s vs %s", a.ID, b.ID)
	}
	c, err := LoginOffline("Bob")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == c.ID {
		t.Errorf("different nicks should yield different UUIDs")
	}
}

func TestLoginOfflineRejectsBadNick(t *testing.T) {
	cases := []string{"", "ab", "a name with spaces", "way_too_long_username_123"}
	for _, n := range cases {
		if _, err := LoginOffline(n); err == nil {
			t.Errorf("expected error for %q", n)
		}
	}
}
