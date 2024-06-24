package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type EventType string

const (
	EVT_TERM  EventType = "term"
	EVT_LOG   EventType = "log"
	EVT_STATE EventType = "state"
	EVT_FLAGS EventType = "flags"
)

// App struct
type App struct {
	ctx     context.Context
	na      *NeoAgent
	naReady sync.WaitGroup

	logBuffer      *bytes.Buffer
	logBufferLimit int

	conf                     Config
	configFilename           string
	disableConfigPersistence bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		logBuffer:      &bytes.Buffer{},
		logBufferLimit: 16 * 1024,
		conf: Config{
			UI: UIOptions{
				Theme: "sl-theme-light",
			},
		},
	}
}

type Config struct {
	UI            UIOptions      `json:"ui"`
	LaunchOptions *LaunchOptions `json:"launchOptions,omitempty"`
}

type UIOptions struct {
	Theme string `json:"theme"`
}

type LaunchOptions struct {
	BinPath     string `json:"binPath,omitempty"`
	Data        string `json:"data,omitempty"`
	File        string `json:"file,omitempty"`
	Host        string `json:"host,omitempty"`
	LogLevel    string `json:"logLevel,omitempty"`
	LogFilename string `json:"logFilename,omitempty"`
	Experiment  bool   `json:"experiment,omitempty"`
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.loadLaunchOptions()

	a.naReady.Add(1)
	defer a.naReady.Done()

	var binPath = ""
	if a.conf.LaunchOptions != nil {
		binPath = a.conf.LaunchOptions.BinPath
	} else {
		// for development
		// wails dev -appargs "<path to machbase-neo>"
		if len(os.Args) > 1 {
			binPath = os.Args[1]
		}
	}
	cwdPath, _ := os.Executable()
	if runtime.GOOS == "darwin" {
		cwdPath = filepath.Join(filepath.Dir(cwdPath), "../../..")
	} else {
		cwdPath = filepath.Dir(cwdPath)
	}
	binName := "machbase-neo"
	displayBinName := "machbase-neo executable file"
	if runtime.GOOS == "windows" {
		binName = "machbase-neo.exe"
		displayBinName = "machbase-neo.exe file"
	}
select_bin_path:
	binPath, err := getMachbaseNeoPath(binPath)
	if err != nil {
		rsp, _ := wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
			Type:          wailsRuntime.ErrorDialog,
			Title:         "Not found",
			Message:       fmt.Sprintf("Can not find %s. Please check the path.", displayBinName),
			Buttons:       []string{"Quit", "Select..."},
			DefaultButton: "Quit",
			CancelButton:  "Quit",
		})
		if rsp == "Select..." {
			binPath, err = wailsRuntime.OpenFileDialog(ctx, wailsRuntime.OpenDialogOptions{
				DefaultDirectory:           cwdPath,
				DefaultFilename:            binName,
				Title:                      "Select " + binName,
				ResolvesAliases:            true,
				TreatPackagesAsDirectories: false,
			})
			if err != nil {
				wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
					Type:    wailsRuntime.ErrorDialog,
					Title:   "Error",
					Message: err.Error(),
				})
			}
			goto select_bin_path
		}
		wailsRuntime.Quit(ctx)
	}
	if a.conf.LaunchOptions.BinPath != binPath {
		a.conf.LaunchOptions.BinPath = binPath
		a.saveLaunchOptions()
	}
	a.ctx = ctx
	a.na = NewNeoAgent(
		WithStdoutWriter(NewAppWriter(a, EVT_TERM)),
		WithStderrWriter(NewAppWriter(a, EVT_TERM)),
		WithLogWriter(NewAppWriter(a, EVT_LOG)),
		WithStateCallback(func(state NeoState) {
			wailsRuntime.EventsEmit(a.ctx, string(EVT_STATE), state)
		}),
		WithLaunchFlags(a.makeLaunchFlags),
	)
}

func (a *App) BeforeClose(ctx context.Context) bool {
	if a.na != nil && a.na.process != nil {
		rsp, err := wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
			Type:          wailsRuntime.QuestionDialog,
			Title:         "Server is running",
			Message:       "Are you sure you want to stop the machbase-neo and quit?",
			Buttons:       []string{"Quit", "Cancel"},
			DefaultButton: "Cancel",
			CancelButton:  "Canel",
		})
		if rsp == "Cancel" && err == nil {
			return true
		}
		a.na.StopServer()
	}
	return false
}

func (a *App) Shutdown(ctx context.Context) {
	a.na.Close()
	a.saveLaunchOptions()
}

func NewAppWriter(app *App, evtType EventType) io.Writer {
	return &AppWriter{
		app:     app,
		evtType: evtType,
	}
}

func (a *App) saveLaunchOptions() {
	if !a.disableConfigPersistence {
		a.conf.LaunchOptions = a.GetLaunchOptions()
		content, _ := json.Marshal(a.conf)
		os.WriteFile(a.configFilename, content, 0644)
	}
}

func (a *App) loadLaunchOptions() {
	if confDir, err := os.UserConfigDir(); err != nil {
		a.disableConfigPersistence = true
	} else {
		confDir = filepath.Join(confDir, "com.machbase.neo-launcher")
		if _, err := os.Stat(confDir); os.IsNotExist(err) {
			os.Mkdir(confDir, 0755)
		}
		a.configFilename = filepath.Join(confDir, "config.json")
		if _, err := os.Stat(a.configFilename); err == nil {
			content, err := os.ReadFile(a.configFilename)
			if err == nil {
				json.Unmarshal(content, &a.conf)
			}
		}
	}
}

func (a *App) appendLog(p []byte) {
	a.logBuffer.Write(p)
	if a.logBuffer.Len() > a.logBufferLimit {
		a.logBuffer.Next(a.logBuffer.Len() - a.logBufferLimit)
	}
}

type AppWriter struct {
	app     *App
	evtType EventType
}

func (w *AppWriter) Write(p []byte) (n int, err error) {
	wailsRuntime.EventsEmit(w.app.ctx, string(w.evtType), string(p))
	w.app.appendLog(p)
	return len(p), nil
}

func (a *App) OnDomReady(ctx context.Context) {
}

func (a *App) DoFrontendReady() {
	a.naReady.Wait()
	if a.na == nil {
		return
	}
	a.na.Open()

	a.emitLaunchCmdWithFlags()

	if a.logBuffer.Len() > 0 {
		wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), a.logBuffer.String())
	} else {
		a.na.Version()
	}
}

func (a *App) emitLaunchCmdWithFlags() {
	v := a.makeLaunchFlags()
	wailsRuntime.EventsEmit(a.ctx, string(EVT_FLAGS), v)
}

type LaunchCmdWithFlags struct {
	BinPath string   `json:"binPath"`
	Flags   []string `json:"flags"`
}

type ProcessInfo struct {
	OS  string `json:"os"`
	PID int    `json:"pid"`
}

func (a *App) DoGetProcessInfo() string {
	ret := ProcessInfo{
		OS:  runtime.GOOS,
		PID: a.na.process.Pid,
	}
	out, _ := json.Marshal(ret)
	result := string(out)
	wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), result)
	return result
}

func (a *App) DoGetOS() string {
	return runtime.GOOS
}

func (a *App) DoGetFlags() {
	cmd := a.makeLaunchFlags()
	strFlags := strings.Join(cmd.Flags, " ")
	wailsRuntime.EventsEmit(a.ctx, string(EVT_FLAGS), strFlags)
}

func (a *App) GetLaunchOptions() *LaunchOptions {
	return a.conf.LaunchOptions
}

func (a *App) SetLaunchOptions(opts *LaunchOptions) {
	if opts == nil {
		return
	}
	a.conf.LaunchOptions = opts
	a.saveLaunchOptions()
	a.emitLaunchCmdWithFlags()
}

func (a *App) makeLaunchFlags() *LaunchCmdWithFlags {
	ret := &LaunchCmdWithFlags{
		BinPath: a.conf.LaunchOptions.BinPath,
		Flags:   []string{},
	}

	if a.conf.LaunchOptions.Data != "" {
		ret.Flags = append(ret.Flags, "--data", a.conf.LaunchOptions.Data)
	}
	if a.conf.LaunchOptions.File != "" {
		ret.Flags = append(ret.Flags, "--file", a.conf.LaunchOptions.File)
	}
	if a.conf.LaunchOptions.Host != "" && a.conf.LaunchOptions.Host != "127.0.0.1" {
		ret.Flags = append(ret.Flags, "--host", a.conf.LaunchOptions.Host)
	}
	if a.conf.LaunchOptions.LogLevel != "INFO" {
		ret.Flags = append(ret.Flags, "--log-level", a.conf.LaunchOptions.LogLevel)
	}
	if a.conf.LaunchOptions.LogFilename != "" && a.conf.LaunchOptions.LogFilename != "-" {
		ret.Flags = append(ret.Flags, "--log-filename", a.conf.LaunchOptions.LogFilename)
	}
	if a.conf.LaunchOptions.Experiment {
		ret.Flags = append(ret.Flags, "--experiment", "true")
	}
	return ret
}

func (a *App) DoOpenBrowser() {
	wailsRuntime.BrowserOpenURL(a.ctx, "http://"+bestGuess.httpAddr)
}

var regexpAnsi = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func (a *App) DoCopyLog() {
	text := regexpAnsi.ReplaceAllString(a.logBuffer.String(), "")
	wailsRuntime.ClipboardSetText(a.ctx, text)
}

func (a *App) DoStartServer() {
	a.na.StartServer()
}

func (a *App) DoStopServer() {
	a.na.StopServer()
}

func (a *App) DoVersion() {
	a.na.Version()
}

func (a *App) DoSetTheme(theme string) {
	a.conf.UI.Theme = theme
}

func (a *App) DoGetTheme() string {
	return a.conf.UI.Theme
}

type guess struct {
	httpAddr string
	grpcAddr string
}

var bestGuess = guess{
	httpAddr: "127.0.0.1:5654",
	grpcAddr: "127.0.0.1:5655",
}

func guessBindAddress(args []string) guess {
	host := "127.0.0.1"
	grpcPort := "5655"
	httpPort := "5654"
	for i := 0; i < len(args); i++ {
		s := args[i]
		if strings.HasPrefix(s, "--host=") {
			host = s[7:]
		} else if s == "--host" && len(args) > i+1 && !strings.HasPrefix(args[i+1], "-") {
			host = args[i+1]
			i++
		} else if strings.HasPrefix(s, "--grpc-port=") {
			grpcPort = s[12:]
		} else if s == "--grpc-port" && len(args) > i+1 && !strings.HasPrefix(args[i+1], "-") {
			grpcPort = args[i+1]
			i++
		} else if strings.HasPrefix(s, "--http-port=") {
			httpPort = s[12:]
		} else if s == "--http-port" && len(args) > i+1 && !strings.HasPrefix(args[i+1], "-") {
			httpPort = args[i+1]
			i++
		}
	}
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return guess{
		httpAddr: fmt.Sprintf("%s:%s", host, httpPort),
		grpcAddr: fmt.Sprintf("%s:%s", host, grpcPort),
	}
}
