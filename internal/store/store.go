package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	texttemplate "text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	// defaultPrefix is the S3 "folder" holding the Pennsieve default templates.
	defaultPrefix = "default"
	// customPrefixFmt is the S3 "folder" holding an organization's branded
	// templates, e.g. custom/O367 for organization id 367.
	customPrefixFmt = "custom/O%d"
)

// TemplateStore fetches the raw template body for a template file, applying the
// organization branding fallback.
type TemplateStore interface {
	// FetchTemplate returns the raw (unrendered) template body for templateFile.
	// When orgId is non-zero and a branded template exists at
	// custom/O{orgId}/{templateFile}, that body is returned; otherwise it falls
	// back to default/{templateFile}.
	FetchTemplate(ctx context.Context, orgId int64, hasOrg bool, templateFile string) ([]byte, error)
}

// S3TemplateStore is a TemplateStore backed by the email-templates S3 bucket.
type S3TemplateStore struct {
	client *s3.Client
	bucket string
}

func NewS3TemplateStore(client *s3.Client, bucket string) *S3TemplateStore {
	return &S3TemplateStore{client: client, bucket: bucket}
}

func (s *S3TemplateStore) FetchTemplate(ctx context.Context, orgId int64, hasOrg bool, templateFile string) ([]byte, error) {
	if hasOrg {
		customKey := fmt.Sprintf(customPrefixFmt, orgId) + "/" + templateFile
		body, err := s.getObject(ctx, customKey)
		switch {
		case err == nil:
			return body, nil
		case isNotFound(err):
			// No branded template for this org; fall through to the default.
		default:
			return nil, err
		}
	}

	defaultKey := defaultPrefix + "/" + templateFile
	body, err := s.getObject(ctx, defaultKey)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("template file %q not found in bucket %s (key %s)", templateFile, s.bucket, defaultKey)
		}
		return nil, err
	}
	return body, nil
}

func (s *S3TemplateStore) getObject(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading template body for key %s: %w", key, err)
	}
	return body, nil
}

// isNotFound reports whether err is an S3 "key does not exist" error. GetObject
// returns *s3types.NoSuchKey for a missing object.
func isNotFound(err error) bool {
	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var notFound *s3types.NotFound
	return errors.As(err, &notFound)
}

// Render applies the request context to the template body and returns the
// resulting HTML. html/template auto-escapes interpolated values so that
// caller-supplied context (e.g. a custom message or organization name) cannot
// inject markup into the email.
func Render(messageId string, body []byte, context map[string]any) (string, error) {
	tmpl, err := template.New(messageId).Parse(string(body))
	if err != nil {
		return "", fmt.Errorf("error parsing template for messageId %s: %w", messageId, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("error rendering template for messageId %s: %w", messageId, err)
	}
	return buf.String(), nil
}

// RenderSubject renders a subject line against the request context. Unlike the
// body, the subject uses text/template (not html/template): a subject is plain
// text, so HTML-escaping would wrongly turn "&" into "&amp;". A subject with no
// "{{" template actions still parses and renders unchanged, so static subjects
// pass through untouched.
func RenderSubject(messageId, subject string, context map[string]any) (string, error) {
	tmpl, err := texttemplate.New(messageId + ":subject").Parse(subject)
	if err != nil {
		return "", fmt.Errorf("error parsing subject for messageId %s: %w", messageId, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("error rendering subject for messageId %s: %w", messageId, err)
	}
	return buf.String(), nil
}
