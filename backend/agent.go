package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type NeoAgent struct {
	makeLaunchFlags  func() *LaunchCmdWithFlags
	stdoutWriter     io.Writer
	stderrWriter     io.Writer
	logWriter        io.Writer
	process          *os.Process
	processWg        sync.WaitGroup
	stateC           chan NeoState
	stateCallback    func(NeoState)
	navelcordEnabled bool
	navelcordLsnr    *net.TCPListener
	navelcord        net.Conn
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

func WithLaunchFlags(fn func() *LaunchCmdWithFlags) Option {
	return func(na *NeoAgent) {
		na.makeLaunchFlags = fn
	}
}

func WithNavelcordEnabled(flag bool) Option {
	return func(na *NeoAgent) {
		na.navelcordEnabled = flag
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

	if na.navelcordEnabled {
		na.runNavelCordServer()
	}
}

func (na *NeoAgent) Close() {
	if na.navelcordLsnr != nil {
		na.navelcordLsnr.Close()
	}
	if na.stateC != nil {
		close(na.stateC)
	}
}

func (na *NeoAgent) StartServer() {
	na.stateC <- NeoStarting

	pname := ""
	pargs := []string{}
	launch := na.makeLaunchFlags()
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, launch.BinPath)
		pargs = append(pargs, "serve")
		pargs = append(pargs, launch.Flags...)
	} else {
		pname = launch.BinPath
		pargs = append(pargs, "serve")
		pargs = append(pargs, launch.Flags...)
	}
	cmd := exec.Command(pname, pargs...)
	cmd.Env = os.Environ()
	if v := na.navelcordEnv(); na.navelcordEnabled && v != "" {
		cmd.Env = append(cmd.Env, v)
	}
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
	na.stateC <- NeoStopping
	if na.navelcord != nil {
		na.navelcord.Close()
	}
	if na.process != nil {
		na.processWg.Wait()
	}
	na.stateC <- NeoStopped
}

func (na *NeoAgent) navelcordEnv() string {
	if na.navelcordLsnr != nil {
		navelPort := strings.TrimPrefix(na.navelcordLsnr.Addr().String(), "127.0.0.1:")
		return fmt.Sprintf("%s=%s", NAVEL_ENV, navelPort)
	}
	return ""
}

func (na *NeoAgent) runNavelCordServer() {
	if lsnr, err := net.Listen("tcp", "127.0.0.1:0"); err == nil {
		na.navelcordLsnr = lsnr.(*net.TCPListener)
	}
	go func() {
		for {
			if conn, err := na.navelcordLsnr.Accept(); err != nil {
				// when launcher is shutdown
				break
			} else {
				na.navelcord = conn
			}
			for {
				hb := Heartbeat{}
				if err := hb.Unmarshal(na.navelcord); err != nil {
					break
				}

				hb.Ack = time.Now().UnixNano()
				if pkt, err := hb.Marshal(); err != nil {
					break
				} else {
					na.navelcord.Write(pkt)
				}
			}
			if na.navelcord != nil {
				na.navelcord.Close()
				na.navelcord = nil
			}
		}
	}()
}

func (na *NeoAgent) Version() {
	pname := ""
	pargs := []string{}
	launch := na.makeLaunchFlags()
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, launch.BinPath)
		pargs = append(pargs, "version")
	} else {
		pname = launch.BinPath
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

const NAVEL_ENV = "NEOSHELL_NAVELCORD"
const NAVEL_STX = 0x4E
const NAVEL_HEARTBEAT = 1

type Heartbeat struct {
	Timestamp int64 `json:"ts"`
	Ack       int64 `json:"ack,omitempty"`
}

func (hb *Heartbeat) Marshal() ([]byte, error) {
	body, err := json.Marshal(hb)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	buf.Write([]byte{NAVEL_STX, NAVEL_HEARTBEAT})
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	buf.Write(hdr)
	buf.Write(body)
	return buf.Bytes(), nil
}

func (hb *Heartbeat) Unmarshal(r io.Reader) error {
	hdr := make([]byte, 2)
	bodyLen := make([]byte, 4)

	n, err := r.Read(hdr)
	if err != nil || n != 2 || hdr[0] != NAVEL_STX || hdr[1] != NAVEL_HEARTBEAT {
		return errors.New("invalid header stx")
	}
	n, err = r.Read(bodyLen)
	if err != nil || n != 4 {
		return errors.New("invalid body length")
	}
	l := binary.BigEndian.Uint32(bodyLen)
	body := make([]byte, l)
	n, err = r.Read(body)
	if err != nil || uint32(n) != l {
		return errors.New("invalid body")
	}
	if err := json.Unmarshal(body, hb); err != nil {
		return fmt.Errorf("invalid format %s", err.Error())
	}
	return nil
}
