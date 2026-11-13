package handler

import "testing"

func TestOnboardingRedirectURL(t *testing.T) {
	cases := map[string]string{
		"/":                             "/signup",
		"http://localhost:3000/":        "http://localhost:3000/signup",
		"https://judge.example.com/app": "https://judge.example.com/signup",
	}
	for success, want := range cases {
		if got := onboardingRedirectURL(success); got != want {
			t.Fatalf("onboardingRedirectURL(%q) = %q, want %q", success, got, want)
		}
	}
}
