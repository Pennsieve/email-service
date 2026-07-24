package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonArg/scanJSON are the encode/decode halves of how map[string]any fields
// (Subscription.Context, Notification.Metadata, NotificationAudit.Details)
// cross the jsonb boundary; these are pure and don't need a live database.

func TestJSONArgNilMapBindsNull(t *testing.T) {
	arg, err := jsonArg(nil)
	require.NoError(t, err)
	assert.Nil(t, arg, "a nil map must bind SQL NULL, not the empty-string literal (\"\"::jsonb is invalid)")
}

func TestJSONArgMarshalsMap(t *testing.T) {
	arg, err := jsonArg(map[string]any{"organizationId": float64(367)})
	require.NoError(t, err)
	require.NotNil(t, arg)
	assert.JSONEq(t, `{"organizationId":367}`, *arg)
}

func TestScanJSONNilRawReturnsNilMap(t *testing.T) {
	m, err := scanJSON(nil)
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestScanJSONRoundTrip(t *testing.T) {
	arg, err := jsonArg(map[string]any{"a": "b", "n": float64(1)})
	require.NoError(t, err)
	m, err := scanJSON([]byte(*arg))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": "b", "n": float64(1)}, m)
}

func TestScanJSONInvalidReturnsError(t *testing.T) {
	_, err := scanJSON([]byte("not json"))
	assert.Error(t, err)
}

func TestStatusOrDefaultDefaultsToUnread(t *testing.T) {
	assert.Equal(t, StatusUnread, statusOrDefault(""))
	assert.Equal(t, StatusRead, statusOrDefault(StatusRead))
}
