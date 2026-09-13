package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

var (
	ErrInvalidToken = errors.New("トークンが無効です")
	ErrWrongKind    = errors.New("この操作を行えるユーザーの種類ではありません")
)

type Verifier interface {
	Verify(ctx context.Context, rawToken string) (Principal, error)
}

type OIDCVerifier struct {
	kind     UserKind
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, kind UserKind, issuer, audience string) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("IdP に接続できません（%s）: %w", issuer, err)
	}
	return &OIDCVerifier{
		kind:     kind,
		verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
	}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (Principal, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return toPrincipal(v.kind, token.Subject), nil
}

func toPrincipal(kind UserKind, subject string) Principal {
	return Principal{Kind: kind, Subject: subject}
}
