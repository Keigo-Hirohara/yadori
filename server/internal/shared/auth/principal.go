package auth

import "context"

type UserKind int

const (
	Booker UserKind = iota + 1
	Operator
)

func (k UserKind) String() string {
	switch k {
	case Booker:
		return "booker"
	case Operator:
		return "operator"
	default:
		return "unknown"
	}
}

type Principal struct {
	Kind    UserKind
	Subject string
}

type contextKey struct{}

func withPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func principalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok
}

func BookerFrom(ctx context.Context) (string, bool) {
	p, ok := principalFrom(ctx)
	if !ok || p.Kind != Booker {
		return "", false
	}
	return p.Subject, true
}

func OperatorFrom(ctx context.Context) (string, bool) {
	p, ok := principalFrom(ctx)
	if !ok || p.Kind != Operator {
		return "", false
	}
	return p.Subject, true
}
