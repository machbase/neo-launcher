package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

type NeoAgent struct {
	binPath         string
	makeLaunchFlags func() []string
	stdoutWriter    io.Writer
	stderrWriter    io.Writer
	logWriter       io.Writer
	process         *os.Process
	processWg       sync.WaitGroup
	stateC          chan NeoState
	stateCallback   func(NeoState)
}

type NeoState string

const (
	NeoStarting NeoState = "starting"
	NeoRunning  NeoState = "running"
	NeoStopping NeoState = "stopping"
	NeoStopped  NeoState = "not running"
)

type Option func(*NeoAgent)

func NewNeoAgent(opts ...Option) *NeoAgent {
	neoAgent := &NeoAgent{
		stateC: make(chan NeoState),
	}
	for _, opt := range opts {
		opt(neoAgent)
	}
	return neoAgent
}

func WithBinPath(binPath string) Option {
	return func(na *NeoAgent) {
		na.binPath = binPath
	}
}

func WithStdoutWriter(writer io.Writer) Option {
	return func(na *NeoAgent) {
		na.stdoutWriter = writer
	}
}

func WithStderrWriter(writer io.Writer) Option {
	return func(na *NeoAgent) {
		na.stderrWriter = writer
	}
}

func WithLogWriter(writer io.Writer) Option {
	return func(na *NeoAgent) {
		na.logWriter = writer
	}
}

func WithStateCallback(cb func(NeoState)) Option {
	return func(na *NeoAgent) {
		na.stateCallback = cb
	}
}

func WithLaunchFlags(fn func() []string) Option {
	return func(na *NeoAgent) {
		na.makeLaunchFlags = fn
	}
}

func (na *NeoAgent) Open() {
	if na.process != nil {
		na.stateC <- NeoRunning
		return
	}
	go func() {
		for state := range na.stateC {
			if na.stateCallback == nil {
				continue
			}
			na.stateCallback(state)
		}
	}()
	na.stateC <- NeoStopped
}

func (na *NeoAgent) Close() {
	if na.stateC != nil {
		close(na.stateC)
	}
}

func (na *NeoAgent) StartServer() {
	na.stateC <- NeoStarting

	pname := ""
	pargs := []string{}
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, na.binPath)
		pargs = append(pargs, "serve")
		pargs = append(pargs, na.makeLaunchFlags()...)
	} else {
		pname = na.binPath
		pargs = append(pargs, "serve")
		pargs = append(pargs, na.makeLaunchFlags()...)
	}
	cmd := exec.Command(pname, pargs...)
	sysProcAttr(cmd)

	if na.stdoutWriter != nil {
		stdout, _ := cmd.StdoutPipe()
		go io.Copy(na.stdoutWriter, stdout)
	}
	if na.stderrWriter != nil {
		stderr, _ := cmd.StderrPipe()
		go io.Copy(na.stderrWriter, stderr)
	}

	if err := cmd.Start(); err != nil {
		na.log(err.Error())
		return
	}
	na.process = cmd.Process

	bestGuess = guessBindAddress(pargs)

	na.processWg.Add(1)
	go func() {
		na.stateC <- NeoRunning
		state, err := na.process.Wait()
		na.processWg.Done()
		if err != nil {
			na.log(fmt.Sprintf("Shutdown failed %s", err.Error()))
		} else {
			na.log(fmt.Sprintf("Shutdown done (exit code: %d)", state.ExitCode()))
		}
		na.process = nil
		na.stateC <- NeoStopped
	}()
}

func (na *NeoAgent) StopServer() {
	if na.process != nil {
		na.stateC <- NeoStopping
		if runtime.GOOS == "windows" {
			// On Windows, sending os.Interrupt to a process with os.Process.Signal is not implemented;
			// it will return an error instead of sending a signal.
			// so, this will not work => na.process.Signal(syscall.SIGINT)
			cmd := exec.Command("cmd.exe", "/c", na.binPath, "shell", "--server", bestGuess.grpcAddr, "shutdown")
			sysProcAttr(cmd)

			if na.stdoutWriter != nil {
				stdout, _ := cmd.StdoutPipe()
				go io.Copy(na.stdoutWriter, stdout)
			}
			if na.stderrWriter != nil {
				stderr, _ := cmd.StderrPipe()
				go io.Copy(na.stderrWriter, stderr)
			}

			if err := cmd.Start(); err != nil {
				na.log(err.Error())
			}
			cmd.Wait()
		} else {
			err := na.process.Signal(os.Interrupt)
			if err != nil {
				na.log(err.Error())
			}
		}
		na.processWg.Wait()
	} else {
		na.stateC <- NeoStopped
	}
}

func (na *NeoAgent) Version() {
	pname := ""
	pargs := []string{}
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, na.binPath)
		pargs = append(pargs, "version")
	} else {
		pname = na.binPath
		pargs = append(pargs, "version")
	}
	cmd := exec.Command(pname, pargs...)
	sysProcAttr(cmd)

	if na.stdoutWriter != nil {
		stdout, _ := cmd.StdoutPipe()
		go io.Copy(na.stdoutWriter, stdout)
	}

	if na.stderrWriter != nil {
		stderr, _ := cmd.StderrPipe()
		go io.Copy(na.stderrWriter, stderr)
	}

	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func (na *NeoAgent) log(msg string, args ...any) {
	if na.logWriter != nil {
		fmt.Fprintln(na.logWriter, append([]any{msg}, args...)...)
	}
}
