package core

import "context"

// FreshBearerValidator is installed by the HTTP authentication boundary for
// bearer-authenticated requests. Durable mint operations call it immediately
// before issuing a credential, so the bearer that authorized the request is
// revalidated rather than relying on an earlier positive cache entry.
type FreshBearerValidator func(context.Context) error

type freshBearerValidatorCtxKey struct{}

func WithFreshBearerValidator(ctx context.Context, validate FreshBearerValidator) context.Context {
	return context.WithValue(ctx, freshBearerValidatorCtxKey{}, validate)
}

// revalidateBearer runs the request's FreshBearerValidator, if it has one.
func revalidateBearer(ctx context.Context) error {
	if validate, ok := FreshBearerValidatorFrom(ctx); ok {
		return validate(ctx)
	}
	return nil
}

func FreshBearerValidatorFrom(ctx context.Context) (FreshBearerValidator, bool) {
	if ctx == nil {
		return nil, false
	}
	validate, ok := ctx.Value(freshBearerValidatorCtxKey{}).(FreshBearerValidator)
	return validate, ok && validate != nil
}
