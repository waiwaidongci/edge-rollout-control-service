package system

import "github.com/google/uuid"

type IDGenerator interface {
	New() string
}

type UUIDGenerator struct{}

func (UUIDGenerator) New() string { return uuid.NewString() }
