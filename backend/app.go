package backend

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	EVT_TERM = "term"
)

// App struct
type App struct {
	ctx context.Context
	na  *NeoAgent
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	var binPath string
	// for development
	// wails dev -appargs "<path to machbase-neo>"
	if len(os.Args) > 1 {
		binPath = os.Args[1]
	}
	binPath, err := getMachbaseNeoPath(binPath)
	if err != nil {
		wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
			Type:    wailsRuntime.ErrorDialog,
			Title:   "Error",
			Message: "Can not find machbase-neo executable. Please check the path.",
			Buttons: []string{"Quit"},
		})
		wailsRuntime.Quit(ctx)
	}

	a.ctx = ctx
	a.na = NewNeoAgent(
		WithBinPath(binPath),
		WithStdoutWriter(a),
		WithStderrWriter(a),
	)
}

func (a *App) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, "\r") {
			line = line + "\n"
		} else {
			line = line + "\r\n"
		}
		wailsRuntime.EventsEmit(a.ctx, EVT_TERM, line)
	}
	return len(p), nil
}

func (a *App) Shutdown(ctx context.Context) {
}

// forntend calls this method when the app is ready
func (a *App) Pronto() {
	a.na.doShowVersion()
}

// Version is called by the frontend to get the version of the backend
func (a *App) Version() {
	a.na.doShowVersion()
}

type NeoAgent struct {
	binPath      string
	stdoutWriter io.Writer
	stderrWriter io.Writer
}

type Option func(*NeoAgent)

func NewNeoAgent(opts ...Option) *NeoAgent {
	neoAgent := &NeoAgent{}
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

func (na *NeoAgent) doShowVersion() {
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

}
