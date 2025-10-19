package recurrence

import "testing"

func TestEngine_GenerateOccurrences(t *testing.T) {
	t.Parallel()

	t.Run("respects weekday selections", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure recurrence generation honors requested weekdays")
	})

	t.Run("clips occurrences to the requested period", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure generation stops at EndsOn or query boundary")
	})

	t.Run("handles timezone normalization", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure occurrences are generated in JST and converted correctly")
	})

	t.Run("links generated occurrences back to their source schedule", func(t *testing.T) {
		t.Parallel()
		t.Skip("TODO: ensure occurrences carry parent schedule identifiers")
	})
}
