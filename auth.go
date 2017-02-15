package tunnel

import "strings"

// Auth holds user and password.
type Auth struct {
	User     string
	Password string
}

// NewAuth creates new auth from string representation "user:password".
func NewAuth(auth string) *Auth {
	if auth == "" {
		return nil
	}

	s := strings.SplitN(auth, ":", 2)
	a := &Auth{
		User: s[0],
	}
	if len(s) > 1 {
		a.Password = s[1]
	}

	return a
}
