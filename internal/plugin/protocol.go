// Package plugin runs godo plugins as WebAssembly.
//
// A plugin is a WASI command: a normal program with a main(), compiled to
// wasm. godo writes one JSON request to its stdin, the plugin writes JSON ops
// to its stdout, and godo answers the ones that need answering. The plugin's
// exit code is the script's exit code.
//
// Being a command rather than a set of exported functions is what keeps the
// contract small. There is no memory-sharing ABI to get right, a plugin can be
// written in any language that targets WASI, and it can be tested outside wasm
// entirely — pipe it a request on stdin and read what it says.
//
// A wasm guest has no filesystem, no network and no way to start a process of
// its own, so a plugin reaches the outside world by asking godo. That is a
// property of the platform, not a policy: godo performs what it is asked,
// because the script doing the asking is the catalog's, and a catalog already
// runs with the shell's full reach.
//
// What is pinned is which plugin, not what it may do — the sha256 in the
// catalog. Where the artifact came from is the question worth answering.
package plugin

// APIVersion is the protocol this build speaks. A plugin that answers with a
// different major refuses to run rather than guessing.
const APIVersion = 1

// Request is the single line godo writes to a plugin's stdin.
//
// There is no mode: a plugin is invoked to run. --preview prints the body
// without starting anything, so nothing here has to describe a dry run. A
// later --predict will add a field for it; plugins ignore fields they do not
// know, so that costs nothing today.
type Request struct {
	API int `json:"api"`
	// Runner is the name the catalog asked for, so one plugin can provide
	// several.
	Runner string `json:"runner"`
	// Body is the script body, verbatim. godo does not expand ${godo:…} for a
	// plugin: a plugin body is not shell text, and the values are right here.
	Body string `json:"body"`
	// Argv holds the matcher's captures; Args the tokens left over.
	Argv map[string]string `json:"argv"`
	Args []string          `json:"args"`
	// Config is the plugin's own block from godo.yaml, verbatim.
	Config map[string]any `json:"config,omitempty"`
}

// Op is one line a plugin writes to its stdout.
type Op struct {
	Op string `json:"op"`

	// exec
	Argv    []string `json:"argv,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Capture bool     `json:"capture,omitempty"`

	// out
	Text string `json:"text,omitempty"`

	// slink
	Src   string `json:"src,omitempty"`
	Dst   string `json:"dst,omitempty"`
	Force bool   `json:"force,omitempty"`

	// fetch
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	// Timeout is in seconds; zero means the host's default.
	Timeout int `json:"timeout,omitempty"`
}

// Result is godo's answer to an op that needs one.
type Result struct {
	// Code is a process exit status, or an HTTP status for a fetch.
	Code   int    `json:"code"`
	OK     bool   `json:"ok"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	// Headers carries a fetch's response headers.
	Headers map[string]string `json:"headers,omitempty"`
	// Base64 carries a fetch's body, encoded.
	//
	// A response body is bytes. Carrying it as a JSON string means anything
	// that is not valid UTF-8 comes out replaced — an image arrives shorter
	// than it left, with nothing raised. Encoding costs a third more on the
	// wire and cannot lose a byte.
	Base64 string `json:"base64,omitempty"`
	// Error is set when godo refused: an unknown op, or a capability the
	// catalog did not grant. It is not a failing command — that is Code.
	Error string `json:"error,omitempty"`
}

// Op names.
const (
	// OpExec runs an argument vector and waits. Needs config proc.exec.
	OpExec = "exec"
	// OpOut writes a line to the host's stdout. Needs no capability.
	OpOut = "out"
	// OpSlink creates a symlink.
	OpSlink = "slink"
	// OpFetch makes an HTTP request.
	//
	// A wasm guest has no sockets, so without this the only way to reach the
	// network is to exec something that has them — curl, or whatever the
	// machine happens to carry. That is the platform dependency a plugin
	// exists to remove, so the request is made here, with the same library on
	// every platform.
	OpFetch = "fetch"
)
