package config

import (
	"fmt"
	"time"
)

type Session struct {
	IdleTimeout time.Duration `yaml:"idle_timeout"`
	Lifetime    time.Duration `yaml:"lifetime"`
	Cookie      Cookie        `yaml:"cookie"`
}

type Cookie struct {
	Name     string       `yaml:"name"`
	Domain   string       `yaml:"domain"`
	HttpOnly *bool        `yaml:"http_only"`
	SameSite SameSiteMode `yaml:"same_site"`
	Secure   *bool        `yaml:"secure"`
	Persist  *bool        `yaml:"persist"`
}

type SameSiteMode string

const (
	SessionSameSiteLax    SameSiteMode = "lax"
	SessionSameSiteStrict SameSiteMode = "strict"
	SessionSameSiteNone   SameSiteMode = "none"
)

func (s SameSiteMode) Validate() error {
	switch s {
	case SessionSameSiteLax, SessionSameSiteStrict, SessionSameSiteNone:
		return nil
	case "":
		return nil
	default:
		return fmt.Errorf("invalid same_site mode: %q (valid: lax, strict, none)", s)
	}
}

func (s *Session) SetDefaults() {
	if s == nil {
		return
	}
	if s.IdleTimeout == 0 {
		s.IdleTimeout = 20 * time.Minute
	}
	if s.Lifetime == 0 {
		s.Lifetime = 24 * time.Hour
	}
	if s.Cookie.Name == "" {
		s.Cookie.Name = "session"
	}
	if s.Cookie.SameSite == "" {
		s.Cookie.SameSite = SessionSameSiteLax
	}
	if s.Cookie.HttpOnly == nil {
		httpOnly := true
		s.Cookie.HttpOnly = &httpOnly
	}
	if s.Cookie.Secure == nil {
		secure := false
		s.Cookie.Secure = &secure
	}
	if s.Cookie.Persist == nil {
		persist := true
		s.Cookie.Persist = &persist
	}
}

func (s *Session) Validate() error {
	if s == nil {
		return nil
	}
	return s.Cookie.SameSite.Validate()
}
