package device

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid device")

type Status string

const (
	StatusPending Status = "pending"
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusRetired Status = "retired"
)

type Device struct {
	ID, Name, HardwareModel, SoftwareVersion string
	Labels                                   map[string]string
	Status                                   Status
	CurrentConfigurationID                   string
	LastHeartbeatAt, CreatedAt, UpdatedAt    time.Time
}
type Group struct {
	ID, Name, Selector, Description string
	CreatedAt, UpdatedAt            time.Time
}
type Filter struct {
	Status                                     Status
	HardwareModel, LabelKey, LabelValue, Query string
	Limit, Offset                              int
}
type Selector struct{ Key, Value string }

func ParseSelector(s string) (Selector, error) {
	p := strings.SplitN(strings.TrimSpace(s), "=", 2)
	if len(p) != 2 || strings.TrimSpace(p[0]) == "" || strings.TrimSpace(p[1]) == "" {
		return Selector{}, ErrInvalid
	}
	return Selector{Key: strings.TrimSpace(p[0]), Value: strings.TrimSpace(p[1])}, nil
}

func (d Device) Validate() error {
	if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.HardwareModel) == "" {
		return ErrInvalid
	}
	return nil
}
func NewDeviceLabels(input map[string]string) map[string]string {
	return CloneLabels(input)
}
func EnsureDeviceLabels(entity *Device) {
	if entity != nil && entity.Labels == nil {
		entity.Labels = NewDeviceLabels(nil)
	}
}
func (d *Device) Heartbeat(at time.Time) {
	if d.Status != StatusRetired {
		d.Status = StatusOnline
		d.LastHeartbeatAt = at.UTC()
		d.UpdatedAt = at.UTC()
	}
}
func (d *Device) Retire() { d.Status = StatusRetired; d.UpdatedAt = time.Now().UTC() }
func (g Group) Validate() error {
	if g.ID == "" || g.Name == "" {
		return ErrInvalid
	}
	return nil
}

type Repository interface {
	CreateDevice(context.Context, Device) error
	UpdateDevice(context.Context, Device) error
	GetDevice(context.Context, string) (Device, error)
	ListDevices(context.Context, Filter) ([]Device, int, error)
	CreateGroup(context.Context, Group) error
	GetGroup(context.Context, string) (Group, error)
	ListGroups(context.Context, int, int) ([]Group, int, error)
	ResolveGroup(context.Context, string) ([]Device, error)
}
