package aiagent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type ProcessRequest struct {
	Command    string
	Args       []string
	Dir        string
	Env        []string
	Input      string
	StdoutLine func([]byte) error
	StderrLine func([]byte)
}
type ProcessResult struct {
	Stdout     []byte
	StderrTail string
	ExitCode   int
}
type AIProcessRunner interface {
	Run(context.Context, ProcessRequest) (ProcessResult, error)
}
type ProcessRunner struct{}

const maxEventBytes = 1024 * 1024
const maxStderrBytes = 64 * 1024

// Run drains both streams concurrently. Separate pipes remain owned by this
// runner, so Wait cannot close them before the final event has been consumed.
func (*ProcessRunner) Run(ctx context.Context, req ProcessRequest) (ProcessResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.Dir
	cmd.Env = req.Env
	cmd.Stdin = strings.NewReader(req.Input)
	configureProcessCancellation(cmd)
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		return ProcessResult{}, err
	}
	defer stdout.Close()
	defer stdoutWriter.Close()
	stderr, stderrWriter, err := os.Pipe()
	if err != nil {
		return ProcessResult{}, err
	}
	defer stderr.Close()
	defer stderrWriter.Close()
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		return ProcessResult{}, fmt.Errorf("start AI process: %w", err)
	}
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	stop := context.AfterFunc(ctx, func() { _ = stdout.Close(); _ = stderr.Close() })
	defer stop()
	var output limitedBuffer
	var tail tailBuffer
	var readErr error
	var errMu sync.Mutex
	setError := func(err error) {
		if err != nil {
			errMu.Lock()
			if readErr == nil {
				readErr = err
			}
			errMu.Unlock()
			cancel()
		}
	}
	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		if req.StdoutLine == nil {
			_, err := io.Copy(&output, stdout)
			if output.exceeded {
				err = fmt.Errorf("AI response exceeds %d bytes", maxResponseBytes)
			}
			setError(err)
			return
		}
		setError(scanLines(stdout, req.StdoutLine))
	}()
	go func() {
		defer readers.Done()
		setError(scanLines(stderr, func(line []byte) error {
			tail.Write(line)
			tail.Write([]byte("\n"))
			if req.StderrLine != nil {
				req.StderrLine(line)
			}
			return nil
		}))
	}()
	waitErr := cmd.Wait()
	done := make(chan struct{})
	go func() { readers.Wait(); close(done) }()
	// A child left behind by a failed provider must not keep readers alive.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = stdout.Close()
		_ = stderr.Close()
		<-done
	}
	result := ProcessResult{Stdout: append([]byte(nil), output.Bytes()...), StderrTail: string(tail.bytes), ExitCode: cmd.ProcessState.ExitCode()}
	if readErr != nil && !errors.Is(readErr, os.ErrClosed) {
		return result, fmt.Errorf("read AI output: %w", readErr)
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, waitErr
}

func scanLines(reader io.Reader, handle func([]byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxEventBytes)
	for scanner.Scan() {
		if err := handle(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type tailBuffer struct{ bytes []byte }

func (b *tailBuffer) Write(p []byte) {
	if len(p) >= maxStderrBytes {
		b.bytes = append(b.bytes[:0], p[len(p)-maxStderrBytes:]...)
		return
	}
	overflow := len(b.bytes) + len(p) - maxStderrBytes
	if overflow > 0 {
		copy(b.bytes, b.bytes[overflow:])
		b.bytes = b.bytes[:len(b.bytes)-overflow]
	}
	b.bytes = append(b.bytes, p...)
}
