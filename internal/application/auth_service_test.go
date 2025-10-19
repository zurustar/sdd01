package application

import "testing"

func TestAuthService_Authenticate(t *testing.T) {
	t.Parallel()

	t.Run("verifies password hashing edge cases", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: cover authenticate with legacy hashes and malformed encodings")
	})

	t.Run("enforces account lockout when configured", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure repeated failures trigger lockout behavior")
	})

	t.Run("issues sessions for valid credentials", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert Authenticate returns session token on success")
	})

	t.Run("rejects disabled accounts", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect Authenticate to surface disabled-account sentinel error")
	})

	t.Run("rejects invalid credentials with sentinel error", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: expect Authenticate to return ErrInvalidCredentials")
	})
}

func TestAuthService_RefreshSession(t *testing.T) {
	t.Parallel()

	t.Run("issues new tokens for valid sessions", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: assert RefreshSession rotates session identifiers")
	})

	t.Run("rejects expired sessions", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure RefreshSession refuses expired sessions")
	})
}
