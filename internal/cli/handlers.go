package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	PathHandlerVar   = "ADBFS_PATH"
	SerialHandlerVar = "ADBFS_SERIAL"
	ModelHandlerVar  = "ADBFS_MODEL"
)

func describeHandlerVars() string {
	var buffer bytes.Buffer
	fmt.Fprintln(&buffer, PathHandlerVar, "	- path of the mountpoint.")
	fmt.Fprintln(&buffer, SerialHandlerVar, "	- serial number of the device.")
	fmt.Fprintln(&buffer, ModelHandlerVar, "	- model of the device, as reported by adb.")
	return buffer.String()
}

// FireHandlers executes each handler in handlers with all occurrences of $key or ${key}
// replaced with values[key]. The map keys should be the *HandlerVar constants.
func FireHandlers(handlers []string, values map[string]string) {
	env := appendToEnv(values)

	for _, handler := range handlers {
		words := strings.Split(handler, " ")

		// Expand each word individually, *after* splitting, in case any values contain spaces.
		expanded := expandHandlerVars(words, values)

		go executeHandlerIgnoringErrors(values[SerialHandlerVar], expanded, env)
	}
}

func expandHandlerVars(strs []string, values map[string]string) (expanded []string) {
	mappingFunc := func(name string) string {
		return values[name]
	}
	for _, str := range strs {
		expanded = append(expanded, os.Expand(str, mappingFunc))
	}
	return
}

func executeHandlerIgnoringErrors(serial string, cmdAndArgs []string, env []string) {
	if len(cmdAndArgs) == 0 {
		return
	}

	commandLine := strings.Join(cmdAndArgs, " ")
	Log.Debugln("running on mount handler:", commandLine)

	cmd := exec.Command(cmdAndArgs[0], cmdAndArgs[1:]...)
	cmd.Env = env
	output, err := cmd.CombinedOutput()

	if err != nil {
		Log.Warn(createHandlerLogMsg(output, commandLine,
			"handler exited with error for %s: %s\n", serial, err))
	} else {
		Log.Debug(createHandlerLogMsg(output, commandLine,
			"handler completed successfully for %s\n", serial))
	}
}

func createHandlerLogMsg(output []byte, commandLine string, format string, args ...interface{}) string {
	var logMsg bytes.Buffer

	outputSummary := string(output)
	if len(outputSummary) > 255 {
		outputSummary = outputSummary[:255]
	}
	outputSummary = strings.TrimSpace(outputSummary)

	fmt.Fprintf(&logMsg, format, args...)
	fmt.Fprintln(&logMsg, "command:", commandLine)

	if outputSummary != "" {
		fmt.Fprintln(&logMsg, "output:", outputSummary)
	}

	return strings.TrimSpace(logMsg.String())
}

// appendToEnv returns os.Environ() with the name=value pairs from vars.
func appendToEnv(vars map[string]string) (env []string) {
	env = make([]string, len(os.Environ()))
	copy(env, os.Environ())

	for k, v := range vars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return
}
