package http

import "testing"

func TestSessionMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("rejects requests without valid session tokens", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure middleware returns 401 for missing or invalid tokens")
	})

	t.Run("attaches authenticated principal to request context", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure middleware injects principal into handler context")
	})
}
