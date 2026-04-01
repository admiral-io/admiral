package authn

type TokenKind string

const (
	TokenKindSession TokenKind = "session"
	TokenKindPAT     TokenKind = "pat"
	TokenKindAGT     TokenKind = "agt"
)

func ValidTokenKind(k string) bool {
	switch TokenKind(k) {
	case TokenKindSession, TokenKindPAT, TokenKindAGT:
		return true
	default:
		return false
	}
}

var AllScopes = []string{
	"app:read", "app:write",
	"env:read", "env:write",
	"var:read", "var:write",
	"cluster:read", "cluster:write",
	"connection:read", "connection:write",
	"state:read", "state:write", "state:admin",
	"token:read", "token:write",
	"runner:exec",
}
