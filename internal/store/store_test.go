package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderEscapesHTML(t *testing.T) {
	// html/template must escape caller-supplied context so it cannot inject markup.
	out, err := Render("m", []byte("Hello {{.name}}"), map[string]any{"name": "<script>x</script>"})
	require.NoError(t, err)
	assert.Equal(t, "Hello &lt;script&gt;x&lt;/script&gt;", out)
}

func TestRenderMissingKeyRendersNoValue(t *testing.T) {
	out, err := Render("m", []byte("Hi {{.missing}}!"), map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, "Hi !", out)
}

func TestRenderParseError(t *testing.T) {
	_, err := Render("m", []byte("Hi {{.name"), map[string]any{})
	assert.Error(t, err)
}

func TestRenderSubjectStaticPassesThrough(t *testing.T) {
	out, err := RenderSubject("m", "Dataset Released", map[string]any{"x": "y"})
	require.NoError(t, err)
	assert.Equal(t, "Dataset Released", out)
}

func TestRenderSubjectInterpolates(t *testing.T) {
	out, err := RenderSubject("m", "Proposal submitted to {{.WorkspaceName}}", map[string]any{"WorkspaceName": "SPARC"})
	require.NoError(t, err)
	assert.Equal(t, "Proposal submitted to SPARC", out)
}

func TestRenderSubjectNotHTMLEscaped(t *testing.T) {
	// A subject is plain text; "&" must stay "&", not become "&amp;".
	out, err := RenderSubject("m", "{{.a}} & {{.b}}", map[string]any{"a": "Tom", "b": "Jerry"})
	require.NoError(t, err)
	assert.Equal(t, "Tom & Jerry", out)
}
