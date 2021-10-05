// Command-line options and utilities used by multiple cmds.
package cli

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var Log *logrus.Logger = logrus.StandardLogger()

func init() {
	kingpin.HelpFlag.Short('h')
}

// Initialize sets the app name. Must be called before flag.Parse()
func Initialize(appName string, baseConfig *BaseConfig) {
	if appName == "" {
		panic("appName cannot be empty")
	}
	kingpin.Version(versionString(appName))

	kingpin.Parse()
	initializeLogger(appName, baseConfig)

	if baseConfig.ServeDebug {
		initializeDebugServer(baseConfig)
	}
}

func initializeLogger(appName string, baseConfig *BaseConfig) {
	Log = baseConfig.createLogger()

	// Redirect standard logger (used by fuse) to our logger.
	log.SetOutput(Log.Writer())
	// Disable standard log timestamps, logrus has its own.
	log.SetFlags(0)

	Log.Println(versionString(appName))
}

func initializeDebugServer(baseConfig *BaseConfig) {
	Log.Debug("starting debug server...")

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0, // Bind to a random port.
	})
	if err != nil {
		Log.Errorln("error starting debug server:", err)
		return
	}

	// Publish basic table of contents.
	template, err := template.New("").Parse(`
		<html><body>
			{{range .}}
				<p><a href="{{.Path}}">{{.Text}}</a></p>
			{{end}}
		</body></html>`)
	if err != nil {
		panic(err)
	}
	toc := []struct {
		Text string
		Path string
	}{
		{"Profiling", "/debug/pprof"},
		{"Download a 30-second CPU profile", "/debug/pprof/profile"},
		{"Download a trace file (add ?seconds=x to specify sample length)", "/debug/pprof/trace"},
		{"Requests", "/debug/requests"},
		{"Event log", "/debug/events"},
	}
	http.HandleFunc("/debug", func(w http.ResponseWriter, req *http.Request) {
		template.Execute(w, toc)
	})

	go func() {
		defer listener.Close()
		if err := http.Serve(listener, nil); err != nil {
			Log.Errorln("debug server error:", err)
			return
		}
	}()

	Log.Printf("debug server listening on http://%s/debug", listener.Addr())
}

func formatFlag(name string, value interface{}) string {
	switch value := value.(type) {
	case bool:
		if value {
			return "--" + name
		}
		return "--no-" + name
	default:
		//		if !reflect.ValueOf(value).IsValid() {
		// Default/zero value.
		return fmt.Sprintf("--%s=%v", name, value)
		//		}
		//		return ""
	}
}
