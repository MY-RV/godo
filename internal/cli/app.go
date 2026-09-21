package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/my-rv/godo"
	"github.com/my-rv/godo/internal/catalog"
	"github.com/my-rv/godo/internal/execshell"
	"github.com/my-rv/godo/internal/plugin"
	"github.com/my-rv/godo/internal/pluginstore"
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
	// PluginStore overrides where artifacts are kept (tests).
	PluginStore *pluginstore.Store
	Executable  func() (string, error)
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
	if mode == modeRunners {
		return a.listRunners(a.nearestCatalog())
	}
	if mode == modePlugins || mode == modePluginsInstall {
		return a.plugins(mode == modePluginsInstall, tokens)
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
	if err := requireEngineVersion(cat, godo.Version); err != nil {
		return err
	}
	root := filepath.Dir(path)
	// The default is the shell you are in. A catalog that names no runner gets
	// this one, which is why godo stops running zsh users under sh.
	runner := a.Runner
	if runner == nil {
		runner = execshell.InheritRunner{Dir: root}
	}
	// Registered here rather than in internal/catalog: internal/execshell
	// imports it, so the engine cannot hold it without an import cycle.
	runners := catalog.NewRunnerRegistry()
	if err := runners.Register(catalog.RunnerInherit, execshell.InheritRunner{Dir: root}); err != nil {
		return err
	}
	if len(cat.Engine.Plugins) > 0 {
		ctx := context.Background()
		rt := plugin.NewRuntime(ctx)
		defer rt.Close(ctx)
		store := a.store()
		resolve := func(digest, source string) (string, error) {
			if store.Has(digest) {
				return store.Path(digest)
			}
			// A file sitting next to the catalog is already here; asking
			// someone to install what they can see is nonsense. Its digest is
			// checked either way, so nothing is skipped but the copying.
			if local := localSource(root, source); local != "" {
				return local, nil
			}
			// Anything else has to be fetched, and a run is not the moment to
			// discover that.
			return "", fmt.Errorf("plugin %s is not installed\n  run: godo -e plugins install", source)
		}
		if err := plugin.Register(ctx, rt, cat, runners, root, a.Stdout, a.Stderr, a.Stdin, resolve); err != nil {
			return err
		}
	}
	// Any other name is a shell the catalog asked for by name. godo does not
	// manage those — it proxies to whatever is on PATH.
	runners.Resolve = func(name catalog.RunnerName) (catalog.Runner, error) {
		r, err := execshell.NativeShell(string(name), root)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", catalog.ErrUnknownRunner, err)
		}
		return r, nil
	}
	eng := catalog.NewEngine(cat, runner, catalog.WithRunners(runners))

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

// requireEngineVersion enforces engine.version against the running binary.
//
// A catalog using something a older godo does not have should say so once,
// clearly, instead of failing later in whatever way that feature happens to
// break. A -dev build compares by its numeric part, so working on godo itself
// is not blocked by its own catalog.
func requireEngineVersion(cat *catalog.Catalog, binary string) error {
	want, ok := cat.Engine.Minimum()
	if !ok {
		return nil
	}
	older, err := update.Newer(binary, want)
	if err != nil {
		return fmt.Errorf("engine.version: %w", err)
	}
	if older {
		return fmt.Errorf("%s needs godo %s or newer; this is %s (godo -e update)", cat.Path, want, binary)
	}
	return nil
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

// listRunners prints what a catalog may put in runner: / # @runner here.
//
// "here" is the point: the native shells are whatever this machine has, so the
// list is a fact about the machine, not about godo.
// plugins lists what a catalog declares, or installs it.
//
// Declaring a plugin and having it on this machine are different things: the
// catalog says which artifact a script needs, the store is where that artifact
// actually is. install is what turns the first into the second.
func (a *App) plugins(install bool, args []string) error {
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
	store := a.store()
	ctx := context.Background()

	// A source typed on the command line is relative to where the person is;
	// one written in the catalog is relative to the catalog.
	if install && len(args) > 0 {
		return a.installNew(ctx, store, cat, path, cwd, args)
	}
	if install {
		return a.installDeclared(ctx, store, cat, filepath.Dir(path))
	}
	return a.listPlugins(store, cat)
}

// installNew adds a plugin the catalog does not declare yet.
//
// The digest is not asked for — it is whatever the artifact turns out to
// contain, computed here and written into the catalog. Asking a person to type
// a hash they cannot check is how wrong hashes get committed.
func (a *App) installNew(ctx context.Context, store pluginstore.Store, cat *catalog.Catalog, path, base string, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("-e plugins install: one source at a time, got %v", args)
	}
	source := args[0]
	digest, stored, err := store.Fetch(ctx, base, source)
	if err != nil {
		return err
	}
	for _, p := range cat.Engine.Plugins {
		if p.SHA256 == digest && p.Source == source {
			fmt.Fprintf(a.Stdout, "already declared: %s\n  %s\n", p.Source, stored)
			return nil
		}
	}

	provides, err := providedRunners(source)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := catalog.UpsertPlugin(src, catalog.PluginEntry{
		Source:   source,
		SHA256:   digest,
		Provides: provides,
		Config:   "proc: {exec: true}",
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "installed %s\n  sha256   %s\n  stored   %s\n  declared %s\n\n",
		source, digest, stored, path)
	fmt.Fprintf(a.Stdout, "It provides %s, and was granted proc.exec.\n", strings.Join(provides, ", "))
	fmt.Fprintln(a.Stdout, "Add fs.mount, fs.slink or time.wall under its config if it needs them.")
	return nil
}

// providedRunners derives the runner name from the artifact's filename.
//
// The name a plugin answers to is the catalog's to choose: nothing inside a
// .wasm declares it, and inventing a manifest format to carry one string would
// be a second contract to maintain. The filename is the honest default, and
// the line it writes is there to be edited.
func providedRunners(source string) ([]string, error) {
	base := filepath.Base(strings.TrimSuffix(source, ".wasm"))
	base = strings.TrimPrefix(base, "godo-")
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil, fmt.Errorf("cannot tell what %q provides; name the artifact <runner>.wasm", source)
	}
	return []string{"runner:" + base}, nil
}

// installDeclared fetches every artifact the catalog names.
func (a *App) installDeclared(ctx context.Context, store pluginstore.Store, cat *catalog.Catalog, base string) error {
	if len(cat.Engine.Plugins) == 0 {
		fmt.Fprintln(a.Stdout, "this catalog declares no plugins")
		return nil
	}
	for _, p := range cat.Engine.Plugins {
		stored, err := store.Ensure(ctx, base, p.SHA256, p.Source)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.Stdout, "%-16s %s\n", strings.Join(p.Provides, " "), stored)
	}
	return nil
}

func (a *App) listPlugins(store pluginstore.Store, cat *catalog.Catalog) error {
	if len(cat.Engine.Plugins) == 0 {
		fmt.Fprintln(a.Stdout, "this catalog declares no plugins")
		return nil
	}
	missing := false
	for _, p := range cat.Engine.Plugins {
		state := "installed"
		if !store.Has(p.SHA256) {
			state = "not installed"
			missing = true
		}
		fmt.Fprintf(a.Stdout, "%-16s %-14s %s\n", strings.Join(p.Provides, " "), state, p.Source)
	}
	if missing {
		fmt.Fprintln(a.Stdout, "\nRun: godo -e plugins install")
	}
	return nil
}

// localSource returns the path of a source that is already on disk, or "".
func localSource(root, source string) string {
	if strings.Contains(source, "://") && !strings.HasPrefix(source, "file://") {
		return ""
	}
	p := strings.TrimPrefix(source, "file://")
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
		return ""
	}
	return p
}

func (a *App) store() pluginstore.Store {
	if a.PluginStore != nil {
		return *a.PluginStore
	}
	return pluginstore.Store{}
}

// nearestCatalog loads the catalog for -e runners, or nil.
//
// The listing is useful outside a repo, so a missing or broken catalog is not
// an error here — it only means there is nothing extra to say about plugins.
func (a *App) nearestCatalog() *catalog.Catalog {
	cwd, err := a.cwd()
	if err != nil {
		return nil
	}
	path, err := catalog.FindFile(cwd)
	if err != nil {
		return nil
	}
	cat, err := catalog.LoadFile(path)
	if err != nil {
		return nil
	}
	return cat
}

func (a *App) listRunners(cat *catalog.Catalog) error {
	fmt.Fprintf(a.Stdout, "%-10s %s  [default]\n", string(catalog.RunnerInherit), execshell.DetectShell())
	fmt.Fprintln(a.Stdout)
	fmt.Fprintln(a.Stdout, "shells found here:")
	found := false
	for _, name := range execshell.KnownShells() {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		found = true
		fmt.Fprintf(a.Stdout, "  %-10s %s\n", name, path)
	}
	if !found {
		fmt.Fprintln(a.Stdout, "  (none)")
	}
	if cat != nil {
		a.printDeclaredPlugins(cat)
	}
	a.printShellCheck()
	return nil
}

// printDeclaredPlugins names runners a catalog expects from plugins, so the
// listing does not read as though those names simply do not exist.
func (a *App) printDeclaredPlugins(cat *catalog.Catalog) {
	if len(cat.Engine.Plugins) == 0 {
		return
	}
	fmt.Fprintln(a.Stdout, "\nfrom plugins declared by this catalog:")
	for _, p := range cat.Engine.Plugins {
		fmt.Fprintf(a.Stdout, "  %-16s %s\n", strings.Join(p.Provides, " "), p.Source)
	}
}

// printShellCheck tells the reader how to confirm which shell they are in.
//
// godo answers that from the parent process, which no command run inside a
// shell can report — running one would only describe the shell godo just
// started. So when the detected shell looks wrong, the check has to happen in
// the reader's own terminal, and this says how.
func (a *App) printShellCheck() {
	fmt.Fprintln(a.Stdout, "\nNot the shell you expected? Run this in your terminal:")
	if runtime.GOOS == "windows" {
		fmt.Fprintln(a.Stdout, "  PowerShell   $PSVersionTable.PSVersion")
		fmt.Fprintln(a.Stdout, "  cmd          echo %COMSPEC%")
		fmt.Fprintln(a.Stdout, "\nThen: set GODO_SHELL=C:\\path\\to\\shell.exe")
		return
	}
	fmt.Fprintln(a.Stdout, "  echo $0            (sh, bash, zsh, dash, ksh)")
	fmt.Fprintln(a.Stdout, "  echo $version      (fish)")
	fmt.Fprintln(a.Stdout, "\nThen: GODO_SHELL=/path/to/shell")
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
	if s.Runner != "" {
		fmt.Fprintf(a.Stdout, "@runner %s\n", s.Runner)
	}
	for _, d := range s.Deps {
		fmt.Fprintf(a.Stdout, "@deps %s\n", d)
	}
	fmt.Fprintf(a.Stdout, "%s:\n", s.Key)
	for _, c := range s.Commands {
		// A body can be many lines — a block scalar, or a file included with
		// ${godo:file(…)} — and every one of them is part of this script.
		for _, line := range strings.Split(strings.TrimRight(c, "\n"), "\n") {
			fmt.Fprintf(a.Stdout, "  %s\n", line)
		}
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
	modeRunners
	modePlugins
	modePluginsInstall
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
	case "runners":
		if len(tokens) > 1 {
			return 0, nil, fmt.Errorf("-e runners: unexpected arguments %v", tokens[1:])
		}
		return modeRunners, nil, nil
	case "plugins":
		switch {
		case len(tokens) == 1:
			return modePlugins, nil, nil
		case tokens[1] == "install":
			return modePluginsInstall, tokens[2:], nil
		default:
			return 0, nil, fmt.Errorf("-e plugins: usage: -e plugins | -e plugins install [source]")
		}
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
  godo -e runners           # runners usable on this machine
  godo -e update            # self-update from GitHub Releases
  godo -e update check      # check only

File: godo.yaml (walk-up). Dialects: package | matcher.
Runners: inherit (default) | a shell by name (see -e runners).
Env: GODO_RELEASES_API overrides GitHub API base for update.`)
}

// EngineUsage writes help for -e/--engine commands.
func EngineUsage(w io.Writer) {
	fmt.Fprintln(w, `godo -e|--engine <command>

Built-in commands (not godo.yaml scripts):
  version         print binary version
  runners         list runners usable here
  plugins         what this catalog declares, and whether it is installed
  plugins install fetch them; with a source, add it and fetch it
  update          download latest GitHub Release for this OS/arch
  update check    report whether an update is available
  help            this help

Shortcuts: --version, --update, --update-check
Env: GODO_RELEASES_API`)
}
