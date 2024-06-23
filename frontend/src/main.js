import './style.css';

import * as App from '../wailsjs/go/backend/App';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';

const termThemeDark = {
    background: '#000000',
    foreground: '#ffffff',
    cursor: '#ffffff',
    selection: 'rgba(255, 255, 255, 0.3)',
    black: '#000000',
    red: '#ff5555',
    green: '#50fa7b',
    yellow: '#f1fa8c',
    blue: '#bd93f9',
    magenta: '#ff79c6',
    cyan: '#8be9fd',
    white: '#bbbbbb',
    brightBlack: '#555555',
    brightRed: '#ff5555',
    brightGreen: '#50fa7b',
    brightYellow: '#f1fa8c',
    brightBlue: '#bd93f9',
    brightMagenta: '#ff79c6',
    brightCyan: '#8be9fd',
    brightWhite: '#ffffff'
}
const termThemeLight = {
    foreground: '#3e3e3e',
    background: '#f4f4f4',
    cursor: '#3f3f3f',
    black: '#3e3e3e',
    brightBlack: '#666666',
    red: '#970b16',
    brightRed: '#de0000',
    green: '#07962a',
    brightGreen: '#87d5a2',
    yellow: '#d6cca5',
    brightYellow: '#e7ddb6',
    blue: '#003e8a',
    brightBlue: '#2e6cba',
    magenta: '#e94691',
    brightMagenta: '#ffa29f',
    cyan: '#89d1ec',
    brightCyan: '#1cfafe',
    white: '#ffffff',
    brightWhite: '#ffffff',
    selectionBackground: '#bbbbbb',
}

const term = new Terminal({
    convertEol: true,
    fontFamily: "Menlo, 'DejaVu Sans Mono', 'Lucida Console', monospace",
    fontSize: 14,
});

const termFitAddon = new FitAddon();
const termWebglAddon = new WebglAddon();
termWebglAddon.onContextLoss(e => {
    termWebglAddon.dispose();
});

term.loadAddon(termFitAddon);
term.loadAddon(termWebglAddon);
term.open(document.getElementById('terminal'));
termFitAddon.fit();

const resize_ob = new ResizeObserver(function (entries) {
    try {
        termFitAddon && termFitAddon.fit();
    } catch (err) {
        console.log(err)
    }
})
resize_ob.observe(document.getElementById('terminal'));

const EVT_TERM = 'term';
const EVT_LOG = 'log';
const EVT_STATE = 'state';
const EVT_FLAGS = 'flags';

const STATE_STARTING = 'starting';
const STATE_RUNNING = 'running';
const STATE_STOPPING = 'stopping';
const STATE_STOPPED = 'not running';

window.runtime.EventsOn(EVT_TERM, (data) => {
    term.write(data);
})
window.runtime.EventsOn(EVT_LOG, (data) => {
    term.write(data);
})
window.runtime.EventsOn(EVT_FLAGS, (data) => {
    let flags = document.getElementById('launchFlags');
    flags.value = data.flags.join(' ');

    let launchCmdWithFlags = document.getElementById('launchCmdWithFlags');
    let fullCmd = data.binPath+' serve '+data.flags.join(' ');
    launchCmdWithFlags.innerText = fullCmd;
})
window.runtime.EventsOn(EVT_STATE, (data) => {
    let launchButton = document.getElementById('launchButton');
    let launchIcon = document.getElementById('launchIcon');
    let launchText = document.getElementById('launchText');
    let stateButton = document.getElementById('stateButton');
    let stateText = document.getElementById('stateText');
    let stateIcon = document.getElementById('stateIcon');
    let openBrowserButton = document.getElementById('openBrowserButton');
    switch (data) {
        case STATE_STARTING:
            launchButton.disabled = true;
            openBrowserButton.disabled = true;
            stateButton.disabled = true;
            stateButton.setAttribute('variant', 'warning');
            stateIcon.setAttribute('name', 'dash-circle')
            break;
        case STATE_RUNNING:
            launchText.innerText = 'Stop machbase-neo'
            launchIcon.setAttribute('name', 'sign-stop')
            launchButton.setAttribute('onclick', 'appStopServer()');
            launchButton.setAttribute('variant', 'danger');
            launchButton.disabled = false;
            stateButton.disabled = false;
            stateButton.setAttribute('variant', 'primary');
            stateIcon.setAttribute('name', 'check-circle')
            openBrowserButton.disabled = false;
            break;
        case STATE_STOPPING:
            launchButton.disabled = true;
            openBrowserButton.disabled = true;
            stateButton.disabled = true;
            stateButton.setAttribute('variant', 'warning');
            stateIcon.setAttribute('name', 'dash-circle')
            break;
        case STATE_STOPPED:
            launchText.innerText = 'machbase-neo serve'
            launchIcon.setAttribute('name', 'rocket-takeoff')
            launchButton.setAttribute('onclick', 'appStartServer()');
            launchButton.setAttribute('variant', 'primary');
            launchButton.disabled = false;
            openBrowserButton.disabled = true;
            stateButton.disabled = true;
            stateButton.setAttribute('variant', 'neutral');
            stateIcon.setAttribute('name', 'dash-circle')
            break;
        default:
            term.write('Unknown state: ' + data + '\r\n');
            break;
    }
    stateText.innerText = data.toUpperCase();
})

// Expose the App.Version function to the window
window.appVersion = function () {
    App.DoVersion();
};

window.appStartServer = function () {
    App.DoStartServer();
};

window.appStopServer = function () {
    App.DoStopServer();
};

window.appOpenBrowser = function () {
    App.DoOpenBrowser();
};

window.appCopyLog = function () {
    App.DoCopyLog();
};

window.appGetProcessInfo = function () {
    App.DoGetProcessInfo();
}

window.setTheme = function (newTheme) {
    const themeIcon = document.getElementById('themeIcon')
    const newIcon = newTheme === 'sl-theme-dark' ? 'sun-fill' : 'moon-fill';
    themeIcon.name = newIcon;

    const oldTheme = newTheme === 'sl-theme-dark' ? 'sl-theme-light' : 'sl-theme-dark';
    document.documentElement.classList.replace(oldTheme, newTheme);

    App.DoSetTheme(newTheme);

    const termTheme = newTheme === 'sl-theme-dark' ? termThemeDark : termThemeLight;

    let container = document.getElementById('terminal');
    container.style.backgroundColor = termTheme.background;
    container.style.borderColor = termTheme.background;
    term.options.theme = { ...termTheme };
    term.refresh(0, term.rows - 1);
}

window.toggleTheme = function () {
    const root = document.getElementsByTagName('html')[0];
    const curTheme = root.classList.contains('sl-theme-dark') ? 'sl-theme-dark' : 'sl-theme-light';
    const newTheme = curTheme === 'sl-theme-dark' ? 'sl-theme-light' : 'sl-theme-dark';
    window.setTheme(newTheme);
}

window.onShowLauncherOptions = function () {
    const drawer = document.getElementById('drawer-options');
    App.GetLaunchOptions().then((options) => {
        drawer.querySelectorAll(".item")
            .forEach((item) => {
                switch (item.getAttribute('name')) {
                    case 'data':
                        item.value = options.data ? options.data : '';
                        break;
                    case 'file':
                        item.value = options.file ? options.file : '';
                        break;
                    case 'host':
                        item.value = options.host ? options.host : '';
                        break;
                    case 'log-level':
                        item.value = options.logLevel ? options.logLevel : 'INFO';
                        break;
                    case 'log-filename':
                        item.value = options.logFilename ? options.logFilename : '-';
                        break;
                    case 'experiment':
                        item.checked = options.experiment
                        break;
                    default:
                        console.log('Unknown option: ' + item.getAttribute('name'));
                        break;
                }
            });
    });
}

window.onHideLauncherOptions = function () {
    const drawer = document.getElementById('drawer-options');
    let options = {
        data: drawer.querySelector(".item[name='data']").value,
        file: drawer.querySelector(".item[name='file']").value,
        host: drawer.querySelector(".item[name='host']").value,
        logLevel: drawer.querySelector(".item[name='log-level']").value,
        logFilename: drawer.querySelector(".item[name='log-filename']").value,
        experiment: drawer.querySelector(".item[name='experiment']").checked,
    };
    App.SetLaunchOptions(options)
        .then(() => {
            drawer.hide()
        })
}

try {
    openBrowserIcon = document.getElementById('openBrowserIcon');
    App.DoGetOS().then((os) => {
        switch (os) {
            case "darwin":
                openBrowserIcon.setAttribute('name', 'browser-safari');
                break;
            case "windows":
                openBrowserIcon.setAttribute('name', 'browser-edge');
                break;
            default:
                openBrowserIcon.setAttribute('name', 'browser-firefox');
                break;
        }
    })
} catch (err) {
    console.log(err);
    openBrowserIcon.setAttribute('name', 'browser-chrome');
}

App.DoGetTheme().then((theme) => {
    window.setTheme(theme);
}).catch((err) => {
    console.log(err);
    window.setTheme('sl-theme-light');
});

App.DoFrontendReady();