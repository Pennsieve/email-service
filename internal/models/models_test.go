package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedupeKeyUsesExplicitId(t *testing.T) {
	r := EmailRequest{MessageId: "m", DedupeId: "abc"}
	assert.Equal(t, "abc:a@x.com", r.DedupeKey("a@x.com"))
	// per-recipient: different recipient -> different key
	assert.NotEqual(t, r.DedupeKey("a@x.com"), r.DedupeKey("b@x.com"))
}

func TestDedupeKeyFallbackIsDeterministic(t *testing.T) {
	// Same logical request (context key order differs) -> same hash.
	r1 := EmailRequest{MessageId: "m", Context: map[string]any{"a": 1.0, "b": "x"}}
	r2 := EmailRequest{MessageId: "m", Context: map[string]any{"b": "x", "a": 1.0}}
	assert.Equal(t, r1.DedupeKey("a@x.com"), r2.DedupeKey("a@x.com"))

	// Different context -> different hash.
	r3 := EmailRequest{MessageId: "m", Context: map[string]any{"a": 2.0, "b": "x"}}
	assert.NotEqual(t, r1.DedupeKey("a@x.com"), r3.DedupeKey("a@x.com"))

	// Different messageId -> different hash.
	r4 := EmailRequest{MessageId: "n", Context: map[string]any{"a": 1.0, "b": "x"}}
	assert.NotEqual(t, r1.DedupeKey("a@x.com"), r4.DedupeKey("a@x.com"))
}

func TestOrganizationIdFromJSON(t *testing.T) {
	// JSON numbers decode to float64; OrganizationId must handle that.
	var req EmailRequest
	require.NoError(t, json.Unmarshal([]byte(`{"context":{"organizationId":367}}`), &req))
	id, ok := req.OrganizationId()
	assert.True(t, ok)
	assert.Equal(t, int64(367), id)
}

func TestOrganizationIdAbsent(t *testing.T) {
	req := EmailRequest{Context: map[string]any{}}
	_, ok := req.OrganizationId()
	assert.False(t, ok)
}

func TestOrganizationIdNilContext(t *testing.T) {
	var req EmailRequest
	_, ok := req.OrganizationId()
	assert.False(t, ok)
}

func TestSubjectOverrideAndDefault(t *testing.T) {
	withOverride := EmailRequest{Context: map[string]any{"subject": "Custom"}}
	assert.Equal(t, "Custom", withOverride.Subject("Default"))

	empty := EmailRequest{Context: map[string]any{"subject": ""}}
	assert.Equal(t, "Default", empty.Subject("Default"), "empty subject falls back to default")

	none := EmailRequest{Context: map[string]any{}}
	assert.Equal(t, "Default", none.Subject("Default"))
}
