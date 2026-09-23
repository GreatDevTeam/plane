package api

import "testing"

func TestMemberName(t *testing.T) {
	cases := []struct {
		name string
		m    Member
		want string
	}{
		{"display name wins", Member{DisplayName: "Alice A", FirstName: "Alice", Email: "a@x.com"}, "Alice A"},
		{"falls back to first/last", Member{FirstName: "Bob", LastName: "Jones", Email: "b@x.com"}, "Bob Jones"},
		{"falls back to email", Member{Email: "c@x.com"}, "c@x.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.Name(); got != tc.want {
				t.Errorf("Name() = %q, want %q", got, tc.want)
			}
		})
	}
}
