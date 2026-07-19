package auth

import "testing"

func TestIsDisposableEmail(t *testing.T) {
	cases := []struct {
		email string
		want  bool
	}{
		// Real providers — must NOT be blocked
		{"user@gmail.com", false},
		{"user@yahoo.com", false},
		{"user@outlook.com", false},
		{"user@hotmail.com", false},
		{"user@icloud.com", false},
		{"user@protonmail.com", false},
		{"sal@salvadorspools.com", false},
		{"user@psiloconvalley.com", false},

		// Known disposable — must be blocked
		{"user@mailinator.com", true},
		{"user@guerrillamail.com", true},
		{"user@yopmail.com", true},
		{"user@tempmail.com", true},
		{"user@trashmail.com", true},
		{"user@immenseignite.info", true},
		{"user@maildrop.cc", true},
		{"user@spam4.me", true},
		{"user@throwaway.email", true},

		// Case insensitivity — must be blocked regardless of case
		{"USER@MAILINATOR.COM", true},
		{"User@Guerrillamail.COM", true},

		// Edge cases — must NOT be blocked
		{"", false},
		{"no-at-sign", false},
		{"@nodomain", false},
		{"user@", false},
	}

	for _, c := range cases {
		got := IsDisposableEmail(c.email)
		if got != c.want {
			t.Errorf("IsDisposableEmail(%q) = %v, want %v", c.email, got, c.want)
		}
	}
}
