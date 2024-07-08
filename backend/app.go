package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

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

	neocatAgent *NeoCatAgent

	logBuffer      *bytes.Buffer
	logBufferLimit int

	conf                     Config
	configFilename           string
	disableConfigPersistence bool
	enableLauncherLog        bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		logBuffer:      &bytes.Buffer{},
		logBufferLimit: 128 * 1024,
		conf: Config{
			UI: UIOptions{
				Theme: "sl-theme-light",
			},
			LaunchOptions: &LaunchOptions{
				Host:     "127.0.0.1",
				LogLevel: "INFO",
			},
			NeoCatOptions: &NeoCatOptions{
				Interval:  "1s",
				DestTable: "EXAMPLE",
				Prefix:    "neocat.",
				InputCPU:  true,
				InputMem:  true,
			},
		},
	}
}

type Config struct {
	UI            UIOptions      `json:"ui"`
	LaunchOptions *LaunchOptions `json:"launchOptions,omitempty"`
	NeoCatOptions *NeoCatOptions `json:"neoCatOptions,omitempty"`
}

type UIOptions struct {
	Theme          string   `json:"theme"`
	RecentDirList  []string `json:"recentDirList,omitempty"`
	RecentFileList []string `json:"recentFileList,omitempty"`
}

type LaunchOptions struct {
	BinPath             string `json:"binPath,omitempty"`
	Data                string `json:"data,omitempty"`
	File                string `json:"file,omitempty"`
	Host                string `json:"host,omitempty"`
	LogLevel            string `json:"logLevel,omitempty"`
	LogFilename         string `json:"logFilename,omitempty"`
	HttpDebug           bool   `json:"httpDebug,omitempty"`
	HttpEnableTokenAuth bool   `json:"httpEnableTokenAuth,omitempty"`
	MqttEnableTokenAuth bool   `json:"mqttEnableTokenAuth,omitempty"`
	MqttEnableTls       bool   `json:"mqttEnableTls,omitempty"`
	JwtAtExpire         string `json:"jwtAtExpire,omitempty"`
	JwtRtExpire         string `json:"jwtRtExpire,omitempty"`
	Experiment          bool   `json:"experiment,omitempty"`
}

type NeoCatOptions struct {
	Interval   string `json:"interval"`
	Prefix     string `json:"prefix"`
	DestTable  string `json:"table"`
	InputCPU   bool   `json:"inputCPU"`
	InputMem   bool   `json:"inputMem"`
	OutputFile string `json:"outputFile,omitempty"`
	Pid        int    `json:"pid"`
	BinPath    string `json:"binPath,omitempty"`
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.loadLaunchOptions()

	a.naReady.Add(1)
	defer a.naReady.Done()

	var binPath = ""
	// for development
	// wails dev -appargs "<path to machbase-neo>"
	if len(os.Args) > 1 {
		binPath = os.Args[1]
	} else {
		binPath = a.conf.LaunchOptions.BinPath
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
		dialogOpts := wailsRuntime.MessageDialogOptions{
			Type:  wailsRuntime.QuestionDialog,
			Title: "Not found",
		}
		openDialogOpts := wailsRuntime.OpenDialogOptions{
			DefaultDirectory:           cwdPath,
			DefaultFilename:            binName,
			Title:                      "Select " + binName,
			ResolvesAliases:            true,
			TreatPackagesAsDirectories: false,
		}
		if runtime.GOOS == "windows" {
			msgLines := []string{
				fmt.Sprintf("Can not find %s.", displayBinName),
				"Do you want to find machbase-neo.exe manually or quit the launcher?",
				"\t\"Yes\" to select manually",
				"\t\"No\" to quit the launcher",
			}
			dialogOpts.Message = strings.Join(msgLines, "\n")
			dialogOpts.Buttons = []string{"Yes", "No"}
			dialogOpts.DefaultButton = "No"
			openDialogOpts.Filters = []wailsRuntime.FileFilter{
				{DisplayName: "Executables", Pattern: "*.exe"},
			}
		} else {
			dialogOpts.Message = fmt.Sprintf("Can not find %s. Please check the path.", displayBinName)
			dialogOpts.Buttons = []string{"Quit", "Select..."}
			dialogOpts.DefaultButton = "Quit"
			dialogOpts.CancelButton = "Quit"
		}
		rsp, _ := wailsRuntime.MessageDialog(ctx, dialogOpts)
		if rsp == "Select..." || rsp == "Yes" {
			binPath, err = wailsRuntime.OpenFileDialog(ctx, openDialogOpts)
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
		WithNavelcordEnabled(true),
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
		if (rsp == "Cancel" || rsp == "No") && err == nil {
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

func (a *App) launcherLog(text string) {
	if !a.enableLauncherLog {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("ERR", err.Error())
		return
	}
	dir := filepath.Dir(exe)
	if runtime.GOOS == "darwin" {
		dir = filepath.Join(filepath.Dir(exe), "../../..")
	}

	logPath := filepath.Join(dir, "neo-launcher.log")
	fd, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("ERR", err.Error())
		return
	}
	defer fd.Close()
	fd.WriteString(time.Now().Format("2006-01-02 15:04:05 ") + text + "\n")
}

func NewAppWriter(app *App, evtType EventType) io.Writer {
	return &AppWriter{
		app:     app,
		evtType: evtType,
	}
}

func (a *App) saveLaunchOptions() {
	if !a.disableConfigPersistence {
		json.MarshalIndent(a.conf, "", "  ")
		content, err := json.MarshalIndent(a.conf, "", "  ")
		if err != nil {
			a.launcherLog(err.Error())
		} else {
			a.launcherLog("write config: " + string(content))
		}

		a.launcherLog("save config: " + a.configFilename)
		if err := os.WriteFile(a.configFilename, content, 0644); err != nil {
			a.launcherLog(err.Error())
		}
	}
}

func (a *App) loadLaunchOptions() {
	if confDir, err := os.UserConfigDir(); err != nil {
		a.launcherLog(err.Error())
		a.disableConfigPersistence = true
	} else {
		confDir = filepath.Join(confDir, "com.machbase.neo-launcher")
		if _, err := os.Stat(confDir); err != nil && os.IsNotExist(err) {
			if err := os.Mkdir(confDir, 0755); err != nil {
				a.launcherLog(err.Error())
				a.disableConfigPersistence = true
				return
			}
		}
		a.configFilename = filepath.Join(confDir, "config.json")
		if _, err := os.Stat(a.configFilename); err == nil {
			if content, err := os.ReadFile(a.configFilename); err == nil {
				if err := json.Unmarshal(content, &a.conf); err != nil {
					a.launcherLog("parse config error: " + err.Error())
				} else {
					a.launcherLog("read config: " + string(content))
				}
			} else {
				a.launcherLog("load config error: " + err.Error())
			}
		}
	}
}

func (a *App) DoRevealConfig() {
	a.revealFile(a.configFilename)
}

func (a *App) DoRevealNeoBin() {
	a.revealFile(a.conf.LaunchOptions.BinPath)
}

func (a *App) revealFile(path string) {
	if path == "" {
		return
	}
	if runtime.GOOS == "darwin" {
		exec.Command("open", "-R", path).Run()
	} else if runtime.GOOS == "windows" {
		exec.Command("explorer", "/select,", path).Run()
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
	wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), result+"\r\n")
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

func (a *App) DoGetNeoCatLauncher() *NeoCatOptions {
	dir := filepath.Dir(a.conf.LaunchOptions.BinPath)
	neocatExe := path.Join(dir, "neocat")
	if runtime.GOOS == "windows" {
		neocatExe += ".exe"
	}
	if _, err := os.Stat(neocatExe); err != nil {
		a.conf.NeoCatOptions.BinPath = ""
	} else {
		a.conf.NeoCatOptions.BinPath = neocatExe
	}
	if a.neocatAgent == nil || a.neocatAgent.cmd == nil {
		a.conf.NeoCatOptions.Pid = 0
	}
	return a.conf.NeoCatOptions
}

func (a *App) DoSetNeoCatLauncher(opt *NeoCatOptions) {
	// preserve current bin path
	opt.BinPath = a.conf.NeoCatOptions.BinPath
	opt.Pid = a.conf.NeoCatOptions.Pid
	a.conf.NeoCatOptions = opt
}

func (a *App) DoStartNeoCat() {
	if a.na == nil {
		wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), "neocat can start only when machbase-neo is running\r\n")
		return
	}
	if a.neocatAgent == nil {
		a.neocatAgent = &NeoCatAgent{navelcordEnabled: true}
	}
	err := a.neocatAgent.Start(a.conf.NeoCatOptions, a.NewLogWriter())
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), "neocat error "+err.Error()+"\r\n")
	}
	a.conf.NeoCatOptions.Pid = a.neocatAgent.cmd.Process.Pid
}

func (a *App) DoStopNeoCat() {
	if a.neocatAgent == nil {
		return
	}
	a.neocatAgent.Stop()
	a.conf.NeoCatOptions.Pid = 0
}

const historyLimit = 10

func addHistory(hist []string, item string) []string {
	for i, h := range hist {
		if h == item {
			if i == 0 {
				return hist
			}
			hist = append(hist[:i], hist[i+1:]...)
			break
		}
	}
	if len(hist) >= historyLimit {
		hist = hist[:historyLimit-1]
	}
	return append([]string{item}, hist...)
}

func (a *App) DoGetLaunchOptions() *LaunchOptions {
	return a.conf.LaunchOptions
}

func (a *App) DoSetLaunchOptions(opts *LaunchOptions) {
	if opts == nil {
		return
	}
	if opts.BinPath == "" {
		// preserve current bin path
		opts.BinPath = a.conf.LaunchOptions.BinPath
	}
	if path := opts.Data; path != "" {
		a.conf.UI.RecentDirList = addHistory(a.conf.UI.RecentDirList, path)
	}
	if path := opts.File; path != "" {
		a.conf.UI.RecentFileList = addHistory(a.conf.UI.RecentFileList, path)
	}
	a.conf.LaunchOptions = opts
	a.saveLaunchOptions()
	a.emitLaunchCmdWithFlags()
}

func (a *App) DoGetRecentDirList() []string {
	return a.conf.UI.RecentDirList
}

func (a *App) DoGetRecentFileList() []string {
	return a.conf.UI.RecentFileList
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
	if a.conf.LaunchOptions.HttpDebug {
		ret.Flags = append(ret.Flags, "--http-debug", "true")
	}
	if a.conf.LaunchOptions.HttpEnableTokenAuth {
		ret.Flags = append(ret.Flags, "--http-enable-token-auth", "true")
	}
	if a.conf.LaunchOptions.MqttEnableTokenAuth {
		ret.Flags = append(ret.Flags, "--mqtt-enable-token-auth", "true")
	}
	if a.conf.LaunchOptions.MqttEnableTls {
		ret.Flags = append(ret.Flags, "--mqtt-enable-tls", "true")
	}
	if a.conf.LaunchOptions.JwtAtExpire != "" && a.conf.LaunchOptions.JwtAtExpire != "5m" {
		ret.Flags = append(ret.Flags, "--jwt-at-expire", a.conf.LaunchOptions.JwtAtExpire)
	}
	if a.conf.LaunchOptions.JwtRtExpire != "" && a.conf.LaunchOptions.JwtRtExpire != "60m" && a.conf.LaunchOptions.JwtRtExpire != "1h" {
		ret.Flags = append(ret.Flags, "--jwt-rt-expire", a.conf.LaunchOptions.JwtRtExpire)
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

func (a *App) DoClearLog() {
	a.logBuffer.Reset()
	wailsRuntime.EventsEmit(a.ctx, string(EVT_TERM), `\033c`)
}

func (a *App) DoSaveLog() {
	filename := fmt.Sprintf("machbase-neo-%s.txt", time.Now().Format("20060102-150405"))
	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename:      filename,
		Title:                "Save log as...",
		Filters:              []wailsRuntime.FileFilter{{DisplayName: "*.txt", Pattern: "*.txt"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), err.Error())
		return
	}
	if path == "" {
		return
	}
	text := regexpAnsi.ReplaceAllString(a.logBuffer.String(), "")
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		wailsRuntime.EventsEmit(a.ctx, string(EVT_LOG), err.Error())
	}
}

func (a *App) DoSelectDirectory(org string) string {
	opts := wailsRuntime.OpenDialogOptions{
		DefaultDirectory:     org,
		Title:                "Select directory",
		CanCreateDirectories: true,
	}
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, opts)
	if err != nil || path == "" {
		return org
	}
	return path
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

func getMachbaseNeoPath(defaultPath string) (string, error) {
	var neoExePath string

	// machbase-neo.exe in the same directory wins always
	if selfPath, err := os.Executable(); err == nil {
		selfDir := filepath.Dir(selfPath)
		if runtime.GOOS == "darwin" {
			selfDir = filepath.Join(selfDir, "../../..")
		}

		if runtime.GOOS == "windows" {
			neoExePath = filepath.Join(selfDir, "machbase-neo.exe")
		} else {
			neoExePath = filepath.Join(selfDir, "machbase-neo")
		}

		if _, err := os.Stat(neoExePath); err == nil {
			return neoExePath, nil
		}
	}

	// otherwise, use the default path
	neoExePath = defaultPath
	if stat, err := os.Stat(neoExePath); err != nil {
		return "", err
	} else {
		if stat.IsDir() {
			return "", os.ErrNotExist
		}
	}

	return neoExePath, nil
}
