package rest

import "context"

// ExportVerifyGoogleIDToken exports verifyGoogleIDToken for unit testing in rest_test.
func ExportVerifyGoogleIDToken(h *AuthHandler, ctx context.Context, idToken string) (*GoogleTokenInfo, error) {
	return h.verifyGoogleIDToken(ctx, idToken)
}
