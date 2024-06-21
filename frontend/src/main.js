import './style.css';

import * as App from '../wailsjs/go/backend/App';
import {Terminal} from '@xterm/xterm';
import {FitAddon} from '@xterm/addon-fit';
import {WebglAddon} from '@xterm/addon-webgl';

const term = new Terminal();
term.options.fontFamily = "Menlo, 'DejaVu Sans Mono', 'Lucida Console', monospace";
const termFitAddon = new FitAddon();
const termWebglAddon = new WebglAddon();
termWebglAddon.onContextLoss(e => {
    termWebglAddon.dispose();
});

term.loadAddon(termFitAddon);
term.loadAddon(termWebglAddon);
term.open(document.getElementById('terminal'));
termFitAddon.fit();

const EVT_TERM = 'term';

window.runtime.EventsOn(EVT_TERM, (data) => {
    term.write(data);
})

// Expose the App.Version function to the window
window.appVersion = function() { 
    term.write('\r\n')
    App.Version(); 
};

// Tell the backend we are ready
App.Pronto();

// function () {
//     Version()
    // Call App.Greet(name)
    // try {
    //         .then((result) => {
    //             term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m $ ' + result + '\r\n');
    //         })
    //         .catch((err) => {
    //             console.error(err);
    //         });
    // } catch (err) {
    //     console.error(err);
    // }
//};
