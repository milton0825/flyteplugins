package svc

import (
	"context"
)

type CommandStatus string

//go:generate mockery -all -case=snake

type ServiceClient interface {
	ExecuteCommand(ctx context.Context, commandStr string, extraArgs interface{}) (interface{}, error)
	KillCommand(ctx context.Context, commandID string) error
	GetCommandStatus(ctx context.Context, commandID string) (CommandStatus, error)
}