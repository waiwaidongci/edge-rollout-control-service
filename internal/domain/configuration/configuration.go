package configuration

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid configuration")

type Status string
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
)

func ValidFormat(f Format) bool { return f == FormatYAML || f == FormatJSON }
func ValidateContent(f Format, s string) error {
	if strings.TrimSpace(s) == "" || !ValidFormat(f) {
		return ErrInvalid
	}
	return nil
}

const (
	StatusDraft      Status = "draft"
	StatusPublished  Status = "published"
	StatusDeprecated Status = "deprecated"
)

type Configuration struct {
	ID, Name, Content, ContentSHA256, ChangeSummary, CreatedBy string
	Format                                                     Format
	Version                                                    int
	CompatibleModels                                           []string
	Variables                                                  map[string]string
	Status                                                     Status
	CreatedAt, UpdatedAt                                       time.Time
}
type Filter struct {
	Status        Status
	Name          string
	Model         string
	Sort          string
	Limit, Offset int
}

func (c Configuration) Validate() error {
	if c.ID == "" || c.Name == "" || c.Version < 1 || strings.TrimSpace(c.Content) == "" {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	CreateConfiguration(context.Context, Configuration) error
	GetConfiguration(context.Context, string) (Configuration, error)
	ListConfigurations(context.Context, Filter) ([]Configuration, int, error)
	UpdateConfigurationStatus(context.Context, string, Status, time.Time) error
}
