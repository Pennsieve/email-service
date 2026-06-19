package journal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// SentAtKey must sort lexicographically in the same order as the underlying
// time, so the RecipientSentAtIndex returns a recipient's emails in time order.
func TestSentAtKeyIsLexicographicallyTimeOrdered(t *testing.T) {
	earlier := SentAtKey(time.Unix(1700000000, 0))
	later := SentAtKey(time.Unix(1700000001, 0))
	muchLater := SentAtKey(time.Unix(2000000000, 0))

	assert.Less(t, earlier, later, "1s later must sort after")
	assert.Less(t, later, muchLater)
	// Fixed width keeps ordering stable across magnitudes (no short-key sorts-first bug).
	assert.Equal(t, len(earlier), len(muchLater))
}

func TestSentAtKeyIsZeroPadded(t *testing.T) {
	assert.Equal(t, "00000000001700000000", SentAtKey(time.Unix(1700000000, 0)))
}
