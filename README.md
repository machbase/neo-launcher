# machbase-neo launcher

This launcher is currently under development to replace the existing `neow` launcher.

- It eliminates the requirement for OpenGL.
- It is designed to be lighter and faster.


## Usage

1. Place the launcher in the same directory with `machbase-neo`
2. Or start launcher with an argument that points the path of `machbase-neo` executable file

### Token authentication

With machbase-neo versions that enable token authentication by default, the launcher
keeps HTTP and MQTT token authentication enabled without adding authentication flags.
In **Advanced options**, choose **false (not recommended)** for
`--http-enable-token-auth` or `--mqtt-enable-token-auth` to explicitly opt out.
The default selection is **true (default)**. The opt-out settings
pass `--http-enable-token-auth false` or `--mqtt-enable-token-auth false`, respectively.
Only disable authentication on trusted networks.

The launcher saves these choices as `httpDisableTokenAuth` and `mqttDisableTokenAuth`.
Legacy `httpEnableTokenAuth` and `mqttEnableTokenAuth` settings are no longer used;
they do not disable authentication. Older machbase-neo versions whose default is
`false` are not made secure by this change; upgrade machbase-neo to use the secure default.

## Developer

1. Check out this repository
2. Run `wails dev -appargs "<path to the executable file of machbase-neo>"`
