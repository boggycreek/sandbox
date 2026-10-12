// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"os/exec"
	"sync"

	"github.com/boggycreek/sandbox/pkg/config"
)

// CommandExecutor abstracts execution of external processes.
type CommandExecutor interface {
	CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd
}

// DefaultCommandExecutor executes commands using the standard library os/exec.
type DefaultCommandExecutor struct{}

// CommandContext implements CommandExecutor using exec.CommandContext.
func (DefaultCommandExecutor) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// CommandExecutorFunc adapts a plain function to the CommandExecutor interface.
type CommandExecutorFunc func(ctx context.Context, name string, args ...string) *exec.Cmd

// CommandContext invokes the underlying function.
func (f CommandExecutorFunc) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return f(ctx, name, args...)
}

type executorContextKey struct{}

// WithCommandExecutor returns a copy of parent ctx with the CommandExecutor embedded.
func WithCommandExecutor(ctx context.Context, executor CommandExecutor) context.Context {
	return context.WithValue(ctx, executorContextKey{}, executor)
}

var (
	defaultExecutorMu sync.RWMutex
	defaultExecutor   CommandExecutor = DefaultCommandExecutor{}
)

// GetDefaultExecutor retrieves the package-level default CommandExecutor in a thread-safe manner.
func GetDefaultExecutor() CommandExecutor {
	defaultExecutorMu.RLock()
	defer defaultExecutorMu.RUnlock()
	if defaultExecutor == nil {
		return DefaultCommandExecutor{}
	}
	return defaultExecutor
}

// SetDefaultExecutor sets the package-level default CommandExecutor in a thread-safe manner and returns a restore function.
func SetDefaultExecutor(executor CommandExecutor) func() {
	defaultExecutorMu.Lock()
	orig := defaultExecutor
	if executor == nil {
		defaultExecutor = DefaultCommandExecutor{}
	} else {
		defaultExecutor = executor
	}
	defaultExecutorMu.Unlock()
	return func() {
		defaultExecutorMu.Lock()
		defaultExecutor = orig
		defaultExecutorMu.Unlock()
	}
}

// SetExecCommandContextForTesting configures command execution mocking for unit tests in a thread-safe manner.
func SetExecCommandContextForTesting(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	if fn == nil {
		return SetDefaultExecutor(DefaultCommandExecutor{})
	}
	return SetDefaultExecutor(CommandExecutorFunc(fn))
}

// CommandExecutorFromContext retrieves the CommandExecutor from ctx, or falls back to the package-level default.
func CommandExecutorFromContext(ctx context.Context) CommandExecutor {
	if ctx != nil {
		if exec, ok := ctx.Value(executorContextKey{}).(CommandExecutor); ok && exec != nil {
			return exec
		}
	}
	return GetDefaultExecutor()
}

// CommandContext creates an *exec.Cmd using the CommandExecutor resolved from ctx or the package default.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return CommandExecutorFromContext(ctx).CommandContext(ctx, name, args...)
}

// execCommandContext is an internal alias for CommandContext to ease transition.
func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return CommandContext(ctx, name, args...)
}

// PodmanRuntime provides container runtime operations with an instance-based CommandExecutor seam.
type PodmanRuntime struct {
	Executor CommandExecutor
}

// NewPodmanRuntime creates a new PodmanRuntime instance with the provided executor.
func NewPodmanRuntime(executor CommandExecutor) *PodmanRuntime {
	if executor == nil {
		executor = DefaultCommandExecutor{}
	}
	return &PodmanRuntime{Executor: executor}
}

// WithContext injects this runtime's executor into the context.
func (r *PodmanRuntime) WithContext(ctx context.Context) context.Context {
	if r != nil && r.Executor != nil {
		return WithCommandExecutor(ctx, r.Executor)
	}
	return ctx
}

// CommandContext creates an *exec.Cmd using this runtime's executor.
func (r *PodmanRuntime) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	if r != nil && r.Executor != nil {
		return r.Executor.CommandContext(ctx, name, args...)
	}
	return CommandContext(ctx, name, args...)
}

// EnsureNetwork creates the bridge network if it does not already exist.
func (r *PodmanRuntime) EnsureNetwork(ctx context.Context, netName string) error {
	return EnsureNetwork(r.WithContext(ctx), netName)
}

// EnsureVolume creates a named Podman volume if it does not already exist.
func (r *PodmanRuntime) EnsureVolume(ctx context.Context, volName string) error {
	return EnsureVolume(r.WithContext(ctx), volName)
}

// EnsureRootlessNetNS initializes and reconciles rootless network namespace state.
func (r *PodmanRuntime) EnsureRootlessNetNS(ctx context.Context) error {
	return EnsureRootlessNetNS(r.WithContext(ctx))
}

// StartAgentContainer provisions and starts the container for an agent.
func (r *PodmanRuntime) StartAgentContainer(ctx context.Context, cfg *config.AgentConfig, paths config.Paths, bpHost string, bpPort int) error {
	return StartAgentContainer(r.WithContext(ctx), cfg, paths, bpHost, bpPort)
}

// StopAgentContainer halts an agent container by name.
func (r *PodmanRuntime) StopAgentContainer(ctx context.Context, containerName string) error {
	return StopAgentContainer(r.WithContext(ctx), containerName)
}

// InspectAgentContainer returns container status information.
func (r *PodmanRuntime) InspectAgentContainer(ctx context.Context, containerName string) (*ContainerInfo, error) {
	return InspectAgentContainer(r.WithContext(ctx), containerName)
}

// ResolveAgentImage resolves agent container image using the 3-tier resolution algorithm.
func (r *PodmanRuntime) ResolveAgentImage(ctx context.Context, input string) (string, bool) {
	return ResolveAgentImage(r.WithContext(ctx), input)
}
