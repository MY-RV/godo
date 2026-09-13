package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/my-rv/godo"
	"github.com/my-rv/godo/internal/catalog"
	"github.com/my-rv/godo/internal/execshell"
	"github.com/my-rv/godo/internal/update"
)

// App is the production CLI (thin orchestration).
//
// Bare tokens are catalog scripts (npm-run style). Tool introspection uses
// flags (--ls, --preview) or -e/--engine for built-in godo commands.
type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Getwd  func() (string, error)
	Runner catalog.Runner
	// UpdateClient overrides release checks (tests).
	UpdateClient *update.Client
	Executable   func() (string, error)
}

// New returns an App wired to process stdio.
func New() *App {
	return &App{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		Getwd:  os.Getwd,
	}
}

// Run executes godo with argv (without program name).
func (a *App) Run(args []string) error {
	mode, tokens, err := ParseFlags(args)
	if err != nil {
		return err
	}
	if mode == modeHelp {
		Usage(a.Stderr)
		return nil
	}
	if mode == modeEngineHelp {
		EngineUsage(a.Stderr)
		return nil
	}
	if mode == modeVersion {
		fmt.Fprintln(a.Stdout, godo.Version)
		return nil
	}
	if mode == modeUpdate || mode == modeUpdateCheck {
		return a.runUpdate(mode == modeUpdateCheck)
	}

	cwd, err := a.cwd()
	if err != nil {
		return err
	}
	path, err := catalog.FindFile(cwd)
	if err != nil {
		return err
	}
	cat, err := catalog.LoadFile(path)
	if err != nil {
		return err
	}
	runner := a.Runner
	if runner == nil {
		runner = execshell.Runner{Dir: filepath.Dir(path)}
	}
	eng := catalog.NewEngine(cat, runner)

	switch mode {
	case modeList:
		return a.list(eng, tokens)
	case modePreview:
		lines, err := eng.PreviewLines(tokens)
		if err != nil {
			return err
		}
		for _, line := range lines {
			fmt.Fprintln(a.Stdout, line)
		}
		return nil
	default:
		return eng.Run(tokens)
	}
}

func (a *App) runUpdate(checkOnly bool) error {
	client := a.UpdateClient
	if client == nil {
		client = &update.Client{}
	}
	tag, url, err := client.Latest()
	if err != nil {
		return err
	}
	newer, err := update.Newer(godo.Version, tag)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "current: %s\nlatest:  %s\n", godo.Version, tag)
	if !newer {
		fmt.Fprintln(a.Stdout, "already up to date")
		return nil
	}
	if checkOnly {
		fmt.Fprintln(a.Stdout, "update available")
		return nil
	}
	if url == "" {
		return fmt.Errorf("update available (%s) but no download URL for this platform", tag)
	}
	exeFn := a.Executable
	if exeFn == nil {
		exeFn = os.Executable
	}
	dest, err := exeFn()
	if err != nil {
		return err
	}
	dest, err = filepath.EvalSymlinks(dest)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "downloading %s → %s\n", url, dest)
	if err := update.Download(client.HTTP, url, dest); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "updated OK — re-run godo --version")
	return nil
}

func (a *App) list(eng *catalog.Engine, tokens []string) error {
	if len(tokens) == 0 {
		for _, s := range eng.ListAll() {
			if s.Doc != "" {
				fmt.Fprintf(a.Stdout, "%s  # %s\n", s.Key, strings.ReplaceAll(s.Doc, "\n", " "))
			} else {
				fmt.Fprintln(a.Stdout, s.Key)
			}
		}
		return nil
	}
	m, err := eng.ListMatches(tokens)
	if err != nil {
		return err
	}
	s := m.Script
	if s.Doc != "" {
		fmt.Fprintln(a.Stdout, s.Doc)
	}
	if s.Dialect != "" {
		fmt.Fprintf(a.Stdout, "@dialect %s\n", s.Dialect)
	}
	for _, d := range s.Deps {
		fmt.Fprintf(a.Stdout, "@deps %s\n", d)
	}
	fmt.Fprintf(a.Stdout, "%s:\n", s.Key)
	for _, c := range s.Commands {
		fmt.Fprintf(a.Stdout, "  %s\n", c)
	}
	return nil
}

func (a *App) cwd() (string, error) {
	if a.Getwd != nil {
		return a.Getwd()
	}
	return os.Getwd()
}

type mode int

const (
	modeRun mode = iota
	modeHelp
	modeEngineHelp
	modeVersion
	modeList
	modePreview
	modeUpdate
	modeUpdateCheck
)

// ParseFlags parses context flags before script tokens.
//
// godo ≈ npm run: bare tokens are catalog scripts.
// -e/--engine <cmd> runs built-in godo commands (version, update, …).
func ParseFlags(args []string) (mode mode, tokens []string, err error) {
	mode = modeRun
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			return modeHelp, nil, nil
		case a == "--version":
			return modeVersion, nil, nil
		case a == "--update-check":
			return modeUpdateCheck, nil, nil
		case a == "--update":
			return modeUpdate, nil, nil
		case a == "-e" || a == "--engine":
			return parseEngineCommand(args[i+1:])
		case a == "--ls":
			mode = modeList
			i++
			return mode, args[i:], nil
		case a == "--preview":
			mode = modePreview
			i++
			return mode, args[i:], nil
		case strings.HasPrefix(a, "-"):
			return 0, nil, fmt.Errorf("unknown flag %q", a)
		default:
			return mode, args[i:], nil
		}
	}
	return mode, nil, nil
}

func parseEngineCommand(tokens []string) (mode mode, rest []string, err error) {
	if len(tokens) == 0 {
		return 0, nil, fmt.Errorf("-e/--engine needs a command (try: -e help)")
	}
	switch tokens[0] {
	case "help", "-h", "--help":
		return modeEngineHelp, nil, nil
	case "version":
		if len(tokens) > 1 {
			return 0, nil, fmt.Errorf("-e version: unexpected arguments %v", tokens[1:])
		}
		return modeVersion, nil, nil
	case "update":
		switch {
		case len(tokens) == 1:
			return modeUpdate, nil, nil
		case len(tokens) == 2 && tokens[1] == "check":
			return modeUpdateCheck, nil, nil
		default:
			return 0, nil, fmt.Errorf("-e update: usage: -e update | -e update check")
		}
	default:
		return 0, nil, fmt.Errorf("unknown engine command %q (try: -e help)", tokens[0])
	}
}

// Usage writes CLI help.
func Usage(w io.Writer) {
	fmt.Fprintln(w, `usage: godo [flags] [script-tokens...]
       godo -e|--engine <command>

Model: bare tokens = catalog scripts (like npm run).
       -e/--engine     = built-in godo commands (not scripts).

Flags:
  --help               help
  --version            version (same as -e version)
  --ls [tokens...]     list scripts / show match
  --preview [tokens...]  print expanded commands; do not run
  --update             same as -e update
  --update-check       same as -e update check
  -e, --engine <cmd>   built-in command (see -e help)

Examples:
  godo test                 # script "test" from godo.yaml
  godo update               # script "update" if defined
  godo -e version           # binary version
  godo -e update            # self-update from GitHub Releases
  godo -e update check      # check only

File: godo.yaml (walk-up). Dialects: package | matcher.
Env: GODO_RELEASES_API overrides GitHub API base for update.`)
}

// EngineUsage writes help for -e/--engine commands.
func EngineUsage(w io.Writer) {
	fmt.Fprintln(w, `godo -e|--engine <command>

Built-in commands (not godo.yaml scripts):
  version         print binary version
  update          download latest GitHub Release for this OS/arch
  update check    report whether an update is available
  help            this help

Shortcuts: --version, --update, --update-check
Env: GODO_RELEASES_API`)
}
