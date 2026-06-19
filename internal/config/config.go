package config

import (
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/pennsieve/email-service/internal/journal"
	"github.com/pennsieve/email-service/internal/mailer"
	"github.com/pennsieve/email-service/internal/store"
	"github.com/pennsieve/email-service/internal/templates"
)

// defaultJournalTTLDays is used when JOURNAL_TTL_DAYS is unset or invalid.
const defaultJournalTTLDays = 90

// Env holds the configuration the handler reads from environment variables.
type Env struct {
	PennsieveDomain string
	TemplateBucket  string
	TemplatesTable  string
	JournalTable    string
	JournalTTLDays  int
}

// LoadEnv reads the handler configuration from the environment. The variable
// names mirror those set in the Terraform lambda definition.
func LoadEnv() Env {
	return Env{
		PennsieveDomain: os.Getenv("PENNSIEVE_DOMAIN"),
		TemplateBucket:  os.Getenv("S3_BUCKET"),
		TemplatesTable:  os.Getenv("TEMPLATES_TABLE"),
		JournalTable:    os.Getenv("JOURNAL_TABLE"),
		JournalTTLDays:  ttlDays(os.Getenv("JOURNAL_TTL_DAYS")),
	}
}

func ttlDays(v string) int {
	if v == "" {
		return defaultJournalTTLDays
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultJournalTTLDays
	}
	return n
}

// Config holds the collaborators the handler needs. Collaborators are built
// lazily from an aws.Config and can be overridden via the Set* methods in
// tests, mirroring the convention used by rehydration-service.
type Config struct {
	Env Env

	awsConfig aws.Config

	templateStore templates.Store
	bodyStore     store.TemplateStore
	mailer        mailer.Mailer
	journal       journal.Journal
}

func NewConfig(awsConfig aws.Config, env Env) *Config {
	return &Config{awsConfig: awsConfig, Env: env}
}

func (c *Config) TemplateStore() templates.Store {
	if c.templateStore == nil {
		c.templateStore = templates.NewDynamoStore(dynamodb.NewFromConfig(c.awsConfig), c.Env.TemplatesTable)
	}
	return c.templateStore
}

// SetTemplateStore overrides the template mapping store (for tests).
func (c *Config) SetTemplateStore(s templates.Store) { c.templateStore = s }

func (c *Config) BodyStore() store.TemplateStore {
	if c.bodyStore == nil {
		c.bodyStore = store.NewS3TemplateStore(s3.NewFromConfig(c.awsConfig), c.Env.TemplateBucket)
	}
	return c.bodyStore
}

// SetBodyStore overrides the S3 template body store (for tests).
func (c *Config) SetBodyStore(s store.TemplateStore) { c.bodyStore = s }

func (c *Config) Mailer() mailer.Mailer {
	if c.mailer == nil {
		c.mailer = mailer.NewSESMailer(ses.NewFromConfig(c.awsConfig), c.Env.PennsieveDomain)
	}
	return c.mailer
}

// SetMailer overrides the mailer (for tests).
func (c *Config) SetMailer(m mailer.Mailer) { c.mailer = m }

func (c *Config) Journal() journal.Journal {
	if c.journal == nil {
		c.journal = journal.NewDynamoJournal(dynamodb.NewFromConfig(c.awsConfig), c.Env.JournalTable)
	}
	return c.journal
}

// SetJournal overrides the email-message-log journal writer (for tests).
func (c *Config) SetJournal(j journal.Journal) { c.journal = j }
