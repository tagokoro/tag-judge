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

func TestSafeNextPath(t *testing.T) {
	cases := map[string]bool{
		"/oauth/authorize?client_id=a&state=b": true,
		"/":                                    true,
		"":                                     false,
		"https://evil.example/":                false,
		"//evil.example/":                      false,
		"/\\evil.example/":                     false,
		"oauth/authorize":                      false,
		"/a\r\nSet-Cookie: x=y":                false,
	}
	for next, want := range cases {
		if got := safeNextPath(next); got != want {
			t.Fatalf("safeNextPath(%q) = %v, want %v", next, got, want)
		}
	}
}
