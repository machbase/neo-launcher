package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

type NeoAgent struct {
	binPath       string
	exeArgs       map[string]string
	exeArgsOnce   sync.Once
	stdoutWriter  io.Writer
	stderrWriter  io.Writer
	logWriter     io.Writer
	process       *os.Process
	processWg     sync.WaitGroup
	stateC        chan NeoState
	stateCallback func(NeoState)
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
		stateC:  make(chan NeoState),
		exeArgs: map[string]string{},
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

func (na *NeoAgent) Open() {
	na.exeArgsOnce.Do(func() {
		dir := filepath.Dir(na.binPath)
		na.exeArgs[FLAG_DATA] = filepath.Join(dir, "machbase_home")
		na.exeArgs[FLAG_FILE] = dir
		na.exeArgs[FLAG_HOST] = "127.0.0.1"
		na.exeArgs[FLAG_LOG_LEVEL] = "INFO"
		na.exeArgs[FLAG_LOG_FILENAME] = "-"
	})
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

const (
	FLAG_DATA         = "data"
	FLAG_FILE         = "file"
	FLAG_HOST         = "host"
	FLAG_LOG_LEVEL    = "log-level"
	FLAG_LOG_FILENAME = "log-filename"
)

var FLAGS = []string{FLAG_DATA, FLAG_FILE, FLAG_HOST, FLAG_LOG_LEVEL, FLAG_LOG_FILENAME}

func (na *NeoAgent) SetFlag(key, value string) {
	na.exeArgs[key] = value
}

func (na *NeoAgent) RemoveFlag(key string) {
	delete(na.exeArgs, key)
}

func (na *NeoAgent) GetFlag(key string) string {
	return na.exeArgs[key]
}

func (na *NeoAgent) GetFlagArgs() []string {
	args := []string{}
	for _, k := range FLAGS {
		if v, ok := na.exeArgs[k]; ok {
			args = append(args, fmt.Sprintf("--%s %s", k, v))
		}
	}
	return args
}

func (na *NeoAgent) StartServer() {
	na.stateC <- NeoStarting

	args := na.GetFlagArgs()

	pname := ""
	pargs := []string{}
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, na.binPath)
		pargs = append(pargs, "serve")
		pargs = append(pargs, args...)
	} else {
		pname = na.binPath
		pargs = append(pargs, "serve")
		pargs = append(pargs, args...)
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
