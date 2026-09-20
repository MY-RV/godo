// Command lines is a godo plugin: a worked example of the protocol.
//
// It provides the runner "lines", whose body is one command per line:
//
//	# @runner lines
//	boot: |
//	  git status
//	  ?pnpm install
//	  echo done
//
// Lines run in order and stop at the first failure, like a script body. A line
// starting with "?" may fail without stopping the rest. ${NAME} is replaced
// with a capture; $1, $2, … with a leftover argument.
//
// Build:
//
//	GOOS=wasip1 GOARCH=wasm go build -o lines.wasm ./examples/plugins/lines
//
// There is nothing wasm-specific in here. It reads stdin, writes stdout, and
// exits with a code — which is why it can be tested as an ordinary program.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// request is godo's opening line. Only the fields this plugin uses.
type request struct {
	API    int               `json:"api"`
	Mode   string            `json:"mode"`
	Runner string            `json:"runner"`
	Body   string            `json:"body"`
	Argv   map[string]string `json:"argv"`
	Args   []string          `json:"args"`
}

type op struct {
	Op      string   `json:"op"`
	Argv    []string `json:"argv,omitempty"`
	Capture bool     `json:"capture,omitempty"`
	Line    string   `json:"line,omitempty"`
}

type result struct {
	Code  int    `json:"code"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

const apiVersion = 1

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	out := json.NewEncoder(os.Stdout)

	req, err := readRequest(in)
	if err != nil {
		fail(err)
	}
	if req.API != apiVersion {
		fail(fmt.Errorf("plugin speaks api %d, godo speaks %d", apiVersion, req.API))
	}

	for n, raw := range strings.Split(req.Body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		mayFail := strings.HasPrefix(line, "?")
		line = strings.TrimSpace(strings.TrimPrefix(line, "?"))

		argv, err := substitute(line, req)
		if err != nil {
			fail(fmt.Errorf("line %d: %w", n+1, err))
		}
		if len(argv) == 0 {
			continue
		}

		// Preview is the plugin's business: godo does not know what a body of
		// this shape would do, so it asks, and this answers by describing the
		// commands rather than running them.
		if req.Mode == "preview" {
			emit(out, previewLine(argv, mayFail))
			continue
		}

		res, err := exec(in, out, argv)
		if err != nil {
			fail(err)
		}
		if res.Error != "" {
			fail(fmt.Errorf("line %d: %s", n+1, res.Error))
		}
		if !res.OK && !mayFail {
			// The command's code becomes the script's, like a shell would.
			os.Exit(res.Code)
		}
	}
	os.Exit(0)
}

func readRequest(in *bufio.Scanner) (request, error) {
	if !in.Scan() {
		if err := in.Err(); err != nil {
			return request{}, err
		}
		return request{}, fmt.Errorf("no request on stdin")
	}
	var req request
	if err := json.Unmarshal(in.Bytes(), &req); err != nil {
		return request{}, fmt.Errorf("unreadable request: %w", err)
	}
	return req, nil
}

// substitute splits a line into argv, replacing ${NAME} with a capture and
// $1, $2, … with a leftover argument. A value with a space stays one argument:
// substitution happens after splitting, never before.
func substitute(line string, req request) ([]string, error) {
	var argv []string
	for _, word := range strings.Fields(line) {
		v, err := expandWord(word, req)
		if err != nil {
			return nil, err
		}
		argv = append(argv, v)
	}
	return argv, nil
}

func expandWord(word string, req request) (string, error) {
	var b strings.Builder
	rest := word
	for {
		i := strings.Index(rest, "$")
		if i < 0 || i == len(rest)-1 {
			break
		}
		b.WriteString(rest[:i])
		rest = rest[i+1:]

		if strings.HasPrefix(rest, "{") {
			end := strings.Index(rest, "}")
			if end < 0 {
				return "", fmt.Errorf("unterminated ${ in %q", word)
			}
			name := rest[1:end]
			v, ok := req.Argv[name]
			if !ok {
				return "", fmt.Errorf("unknown capture %q", name)
			}
			b.WriteString(v)
			rest = rest[end+1:]
			continue
		}

		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			b.WriteString("$")
			continue
		}
		n, _ := strconv.Atoi(rest[:digits])
		if n < 1 || n > len(req.Args) {
			return "", fmt.Errorf("$%d out of range (%d args)", n, len(req.Args))
		}
		b.WriteString(req.Args[n-1])
		rest = rest[digits:]
	}
	b.WriteString(rest)
	return b.String(), nil
}

func previewLine(argv []string, mayFail bool) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t'\"") {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			quoted[i] = a
		}
	}
	line := strings.Join(quoted, " ")
	if mayFail {
		line += "    # may fail"
	}
	return line
}

func exec(in *bufio.Scanner, out *json.Encoder, argv []string) (result, error) {
	if err := out.Encode(op{Op: "exec", Argv: argv}); err != nil {
		return result{}, err
	}
	if !in.Scan() {
		if err := in.Err(); err != nil {
			return result{}, err
		}
		return result{}, fmt.Errorf("godo closed the connection")
	}
	var res result
	if err := json.Unmarshal(in.Bytes(), &res); err != nil {
		return result{}, fmt.Errorf("unreadable result: %w", err)
	}
	return res, nil
}

func emit(out *json.Encoder, line string) {
	if err := out.Encode(op{Op: "emit", Line: line}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lines:", err)
	os.Exit(1)
}
