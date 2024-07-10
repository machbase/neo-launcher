package backend

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type NeoCatAgent struct {
	cmd  *exec.Cmd
	host string

	navelcordEnabled bool
	navelcordLsnr    *net.TCPListener
	navelcord        net.Conn
}

func (nc *NeoCatAgent) Start(opt *NeoCatOptions, logWriter io.Writer) error {
	if nc.navelcordEnabled {
		nc.runNavelCordServer()
	}
	if nc.host == "" {
		nc.host = "127.0.0.1:5653"
	}
	args := []string{}
	if opt.Interval != "" {
		args = append(args, "--interval", opt.Interval)
	}
	if opt.Prefix != "" {
		args = append(args, "--tag-prefix", opt.Prefix)
	}
	if opt.InputCPU {
		args = append(args, "--in-cpu")
	}
	if opt.InputMem {
		args = append(args, "--in-mem")
	}

	if opt.DestTable != "" {
		args = append(args, "--out-mqtt", fmt.Sprintf("tcp://%s/db/append/%s:csv", nc.host, opt.DestTable))
	}
	if opt.OutputFile != "" {
		args = append(args, "--out-file", opt.OutputFile)
	}

	args = append(args, "--log-filename", "-")
	args = append(args, "--log-level", "DEBUG")

	nc.cmd = exec.Command(opt.BinPath, args...)
	nc.cmd.Env = os.Environ()
	if v := nc.navelcordEnv(); nc.navelcordEnabled && v != "" {
		nc.cmd.Env = append(nc.cmd.Env, v)
	}
	sysProcAttr(nc.cmd)

	stdout, _ := nc.cmd.StdoutPipe()

	if logWriter != nil {
		go io.Copy(logWriter, stdout)
	}
	err := nc.cmd.Start()
	return err
}

func (nc *NeoCatAgent) Stop() {
	if nc.cmd != nil {
		nc.cmd.Process.Kill()
	}
	if nc.navelcordLsnr != nil {
		nc.navelcordLsnr.Close()
	}
}

func (nc *NeoCatAgent) navelcordEnv() string {
	if nc.navelcordLsnr != nil {
		navelPort := strings.TrimPrefix(nc.navelcordLsnr.Addr().String(), "127.0.0.1:")
		return fmt.Sprintf("%s=%s", NAVEL_ENV, navelPort)
	}
	return ""
}

func (na *NeoCatAgent) runNavelCordServer() {
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

func (a *App) NewLogWriter() io.Writer {
	return &LogWriter{ctx: a.ctx}
}

type LogWriter struct {
	ctx context.Context
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	wailsRuntime.EventsEmit(lw.ctx, string(EVT_LOG), string(p))
	return len(p), nil
}
