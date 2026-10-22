package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMakeLaunchFlagsTokenAuth(t *testing.T) {
	tests := []struct {
		name string
		http bool
		mqtt bool
		want []string
	}{
		{name: "enabled by default", want: []string{}},
		{name: "disable HTTP", http: true, want: []string{"--http-enable-token-auth", "false"}},
		{name: "disable MQTT", mqtt: true, want: []string{"--mqtt-enable-token-auth", "false"}},
		{name: "disable both", http: true, mqtt: true, want: []string{"--http-enable-token-auth", "false", "--mqtt-enable-token-auth", "false"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			app.conf.LaunchOptions.HttpDisableTokenAuth = tt.http
			app.conf.LaunchOptions.MqttDisableTokenAuth = tt.mqtt
			if got := app.makeLaunchFlags().Flags; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("flags = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenAuthConfigRoundTrip(t *testing.T) {
	for _, config := range []string{
		`{}`,
		`{"httpEnableTokenAuth":true,"mqttEnableTokenAuth":true}`,
		`{"httpEnableTokenAuth":false,"mqttEnableTokenAuth":false}`,
		`{"httpDisableTokenAuth":false,"mqttDisableTokenAuth":false}`,
		`{"httpDisableTokenAuth":true}`,
		`{"mqttDisableTokenAuth":true}`,
		`{"httpDisableTokenAuth":true,"mqttDisableTokenAuth":true}`,
		`{"httpEnableTokenAuth":true,"mqttEnableTokenAuth":true,"httpDisableTokenAuth":true,"mqttDisableTokenAuth":true}`,
	} {
		t.Run(config, func(t *testing.T) {
			app := NewApp()
			if err := json.Unmarshal([]byte(config), app.conf.LaunchOptions); err != nil {
				t.Fatal(err)
			}
			var fields map[string]bool
			if err := json.Unmarshal([]byte(config), &fields); err != nil {
				t.Fatal(err)
			}
			opts := app.DoGetLaunchOptions()
			if opts.HttpDisableTokenAuth != fields["httpDisableTokenAuth"] ||
				opts.MqttDisableTokenAuth != fields["mqttDisableTokenAuth"] {
				t.Fatalf("unexpected token auth options: %+v", opts)
			}
			wantFields := map[string]bool{}
			if opts.HttpDisableTokenAuth {
				wantFields["httpDisableTokenAuth"] = true
			}
			if opts.MqttDisableTokenAuth {
				wantFields["mqttDisableTokenAuth"] = true
			}
			data, err := json.Marshal(&LaunchOptions{
				HttpDisableTokenAuth: opts.HttpDisableTokenAuth,
				MqttDisableTokenAuth: opts.MqttDisableTokenAuth,
			})
			if err != nil {
				t.Fatal(err)
			}
			var savedFields map[string]bool
			if err := json.Unmarshal(data, &savedFields); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(savedFields, wantFields) {
				t.Fatalf("saved fields = %v, want %v", savedFields, wantFields)
			}
			reloaded := NewApp()
			if err := json.Unmarshal(data, reloaded.conf.LaunchOptions); err != nil {
				t.Fatal(err)
			}
			if got, want := reloaded.makeLaunchFlags(), app.makeLaunchFlags(); !reflect.DeepEqual(got, want) {
				t.Fatalf("reloaded command = %v, want %v", got, want)
			}
		})
	}
}

func TestTokenAuthOptionsPersistence(t *testing.T) {
	app := NewApp()
	app.configFilename = filepath.Join(t.TempDir(), "config.json")
	app.conf.LaunchOptions.HttpDisableTokenAuth = true
	app.conf.LaunchOptions.MqttDisableTokenAuth = true
	app.saveLaunchOptions()

	data, err := os.ReadFile(app.configFilename)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := NewApp()
	if err := json.Unmarshal(data, &reloaded.conf); err != nil {
		t.Fatal(err)
	}
	if got, want := reloaded.makeLaunchFlags(), app.makeLaunchFlags(); !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded command = %v, want %v", got, want)
	}
	reloaded.conf.LaunchOptions.HttpDisableTokenAuth = false
	reloaded.conf.LaunchOptions.MqttDisableTokenAuth = false
	if got := reloaded.makeLaunchFlags().Flags; len(got) != 0 {
		t.Fatalf("re-enabled authentication must omit flags, got %v", got)
	}
}
