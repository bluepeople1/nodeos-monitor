package nodeosmonitor

import (
	"context"
	"os/exec"
	"sync"

	"bufio"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	_ Monitorable = &ProcessMonitor{}
)

// Monitorable is a type that can be activated and shutdown. In this
// codebase, a Monitorable is usually an actual OS process.
type Monitorable interface {
	Activate(context.Context, ProcessFailureHandler) error
	Shutdown(context.Context) error
}

// ProcessFailureHandler is something that's called when a process
// fails.
type ProcessFailureHandler interface {
	HandleFailure(ctx context.Context, m Monitorable)
}

// ProcessMonitor monitors a process informing a handler when the
// process fails.
type ProcessMonitor struct {
	mutex    *sync.Mutex
	path     string
	args     []string
	cmd      *exec.Cmd
	shutdown bool
}

// NewProcessMonitor returns a new instance of ProcessMonitor.
func NewProcessMonitor(path string, args []string) *ProcessMonitor {
	return &ProcessMonitor{
		mutex: &sync.Mutex{},
		path:  path,
		args:  args,
	}
}

func manageWrappedProcessOut(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrapf(err, "error opening stdout pipe")
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			logrus.Infof("wrapped process stdout: %s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			logrus.WithError(err).Errorf("error scanning stdout")
		}
	}()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrapf(err, "error opening stdout pipe")
	}
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logrus.Infof("wrapped process stderr: %s", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			logrus.WithError(err).Errorf("error scanning stderr")
		}
	}()

	return nil
}

// IsActive returns true if the underlying process is active.
func (p *ProcessMonitor) IsActive() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.cmd != nil
}

// Activate starts the underlying process.
func (p *ProcessMonitor) Activate(ctx context.Context,
	failureHandler ProcessFailureHandler) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	logrus.Debugf("activating process %v", p.path)

	if p.cmd != nil {
		return errors.New("error: process is already active")
	}

	cmd := exec.CommandContext(ctx, p.path, p.args...)

	// Print all stdin/stderr output to the logrus logger.
	if err := manageWrappedProcessOut(cmd); err != nil {
		return errors.Wrapf(err, "error managing wrapped process output")
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "error starting command")
	}
	p.cmd = cmd
	p.shutdown = false

	// Create exited channel so that we can wait for this in next
	// goroutine.
	exitedChan := make(chan struct{})
	go func() {
		if err := cmd.Wait(); err != nil {
			logrus.WithError(err).Errorf("error waiting for command to finish executing")
		}
		close(exitedChan)
	}()

	// This goroutine waits until the process fails, notifying the
	// failure handler.
	go func() {
		select {
		case <-exitedChan:
		case <-ctx.Done():
			return
		}

		logrus.Debugf("detected process failure %v", p.path)

		// Check that this is a random failure, not a shutdown.
		p.mutex.Lock()
		if p.shutdown {
			return
		}
		p.mutex.Unlock()

		failureHandler.HandleFailure(ctx, p)

		if err := p.Shutdown(ctx); err != nil {
			logrus.WithError(err).Errorf("error shutting down process")
		}
	}()

	return nil
}

// Shutdown shuts the current process down.
func (p *ProcessMonitor) Shutdown(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	logrus.Debugf("shutting down process %v", p.path)

	// This is to let goroutines know that this isn't a random
	// failure.
	p.shutdown = true

	if p.cmd != nil && p.cmd.ProcessState == nil {
		logrus.Debugf("killing process %v", p.path)

		if err := p.cmd.Process.Kill(); err != nil {
			// TODO: maybe do a kill -9 here if the process doesn't
			// shut down cleanly?
			return errors.Wrapf(err, "error killing the process")
		}

		logrus.Debugf("killed process %v", p.path)
	}

	p.cmd = nil

	logrus.Debugf("shut down process %v", p.path)

	return nil
}
