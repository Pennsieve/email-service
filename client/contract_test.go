package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests lock the Go client to the shared wire contract in
// ../contract/fixtures. The Scala client tests against the same files, so a
// shape change in either language fails CI here or there.

const fixturesDir = "../contract/fixtures"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixturesDir, name))
	require.NoError(t, err, "read fixture %s", name)
	return b
}

// decodeJSON normalizes to a generic structure so comparisons ignore key order
// and formatting.
func decodeJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

// Every fixture must round-trip through EmailRequest unchanged — this is the
// shape the consumer parses, so it must accept all of them.
func TestFixturesRoundTrip(t *testing.T) {
	for _, name := range []string{
		"minimal.json",
		"with-organization.json",
		"with-dedupe-id.json",
		"multi-recipient.json",
	} {
		t.Run(name, func(t *testing.T) {
			raw := readFixture(t, name)

			var req EmailRequest
			require.NoError(t, json.Unmarshal(raw, &req), "consumer must parse the fixture")
			require.NotEmpty(t, req.MessageId)
			require.NotEmpty(t, req.Recipients)

			// Re-marshal and compare structurally with the original.
			out, err := json.Marshal(req)
			require.NoError(t, err)
			assert.Equal(t, decodeJSON(t, raw), decodeJSON(t, out),
				"re-encoding must match the fixture structurally")
		})
	}
}

// A builder must produce exactly the documented wire shape.
func TestBuilderMatchesFixture(t *testing.T) {
	req := DatasetPublicationAccepted(
		To{Name: "Alice", Email: "alice@example.com"},
		DatasetPublicationAcceptedArgs{DatasetName: "My Dataset", ReviewerName: "Bob", Date: "2026-06-18"},
	)
	out, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Equal(t, decodeJSON(t, readFixture(t, "minimal.json")), decodeJSON(t, out))

	withOrg := req.WithOrganization(367)
	out, err = json.Marshal(withOrg)
	require.NoError(t, err)
	assert.Equal(t, decodeJSON(t, readFixture(t, "with-organization.json")), decodeJSON(t, out))
}
