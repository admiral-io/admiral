package config

import (
	"fmt"
	"strings"
	"time"
)

type Authn struct {
	Name              string        `yaml:"name"`
	Issuer            string        `yaml:"issuer"`
	ClientID          string        `yaml:"client_id"`
	ClientSecret      string        `yaml:"client_secret"`
	Scopes            []string      `yaml:"scopes"`
	RedirectURL       string        `yaml:"redirect_url"`
	SigningSecret     string        `yaml:"signing_secret"`
	SessionRefreshTTL time.Duration `yaml:"session_refresh_ttl"`
	SkipTLSVerify     bool          `yaml:"skip_tls_verify"`
}

func (a *Authn) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type rawAuthn Authn
	raw := rawAuthn{
		Scopes: []string{"openid", "email", "profile", "offline_access"},
	}

	var temp map[string]interface{}
	if err := unmarshal(&temp); err != nil {
		return err
	}

	if v, ok := temp["name"]; ok && v != nil {
		if s, ok := v.(string); ok {
			raw.Name = s
		}
	}

	if v, ok := temp["issuer"]; ok && v != nil {
		if s, ok := v.(string); ok {
			raw.Issuer = s
		}
	}

	if v, ok := temp["client_id"]; ok && v != nil {
		if s, ok := v.(string); ok {
			raw.ClientID = s
		}
	}

	if v, ok := temp["client_secret"]; ok && v != nil {
		if s, ok := v.(string); ok {
			raw.ClientSecret = s
		}
	}

	if v, ok := temp["redirect_url"]; ok && v != nil {
		if s, ok := v.(string); ok {
			raw.RedirectURL = s
		}
	}

	if v, ok := temp["signing_secret"]; ok && v != nil {
		if s, ok := v.(string); ok {
			raw.SigningSecret = s
		}
	}

	if v, ok := temp["skip_tls_verify"]; ok && v != nil {
		if b, ok := v.(bool); ok {
			raw.SkipTLSVerify = b
		}
	}

	if v, ok := temp["session_refresh_ttl"]; ok && v != nil {
		switch ttl := v.(type) {
		case string:
			if duration, err := time.ParseDuration(ttl); err == nil {
				raw.SessionRefreshTTL = duration
			}
		case float64:
			raw.SessionRefreshTTL = time.Duration(ttl) * time.Second
		case int:
			raw.SessionRefreshTTL = time.Duration(ttl) * time.Second
		}
	}

	// Handle scopes - can be string (comma-separated) or array
	if v, ok := temp["scopes"]; ok && v != nil {
		switch scopes := v.(type) {
		case string:
			if scopes != "" {
				raw.Scopes = strings.Split(scopes, ",")
				for i, scope := range raw.Scopes {
					raw.Scopes[i] = strings.TrimSpace(scope)
				}
			}
		case []interface{}:
			raw.Scopes = make([]string, len(scopes))
			for i, scope := range scopes {
				if scope != nil {
					if s, ok := scope.(string); ok {
						raw.Scopes[i] = s
					}
				}
			}
		}
	}

	*a = Authn(raw)
	return nil
}

func (a *Authn) SetDefaults() {
	if a == nil {
		return
	}

	if len(a.Scopes) == 0 {
		a.Scopes = []string{"openid", "email", "profile", "offline_access"}
	}

	if a.SessionRefreshTTL == 0 {
		a.SessionRefreshTTL = time.Hour * 12
	}
}

func (a *Authn) Validate() error {
	if a == nil {
		return nil
	}

	if a.Issuer == "" {
		return fmt.Errorf("issuer is required")
	}

	if a.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}

	if a.ClientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}

	if len(a.SigningSecret) < 32 {
		return fmt.Errorf("signing_secret must be at least 32 bytes (got %d)", len(a.SigningSecret))
	}

	if a.RedirectURL == "" {
		return fmt.Errorf("redirect_url is required")
	}

	return nil
}
