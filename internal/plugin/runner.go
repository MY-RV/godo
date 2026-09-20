package plugin

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/my-rv/godo/internal/catalog"
)

// Runner adapts a loaded plugin to one runner name.
//
// One plugin may provide several runners; the name travels in the request so
// the plugin can tell which one it was asked for.
type Runner struct {
	Plugin *Plugin
	Name   catalog.RunnerName
	Host   Host
}

// Run implements catalog.Runner.
//
// It always fails: a rendered line has lost the values a plugin reads by name.
// The engine hands a plugin the bound invocation instead.
func (r Runner) Run(string) error {
	return fmt.Errorf("runner %q is a plugin and takes the bound invocation, not a rendered line", r.Name)
}

// AcceptsArgs implements catalog.ArgsAwareRunner.
//
// A plugin body is a program, not a shell template, so there is no
// ${godo:args…} to look for. Whether the leftover tokens mean anything is the
// plugin's business; they are handed over either way.
func (Runner) AcceptsArgs([]string) bool { return true }

// RunInvocation implements catalog.InvocationRunner.
func (r Runner) RunInvocation(inv catalog.Invocation) error {
	out, err := r.Plugin.Invoke(context.Background(), Request{
		Runner: string(r.Name),
		Body:   inv.Body,
		Argv:   nonNil(inv.Captures),
		Args:   inv.Args,
	}, r.Host)
	if err != nil {
		return err
	}
	if out.Code != 0 {
		return &catalog.ExitError{
			Code:    out.Code,
			Message: fmt.Sprintf("script %q failed under runner %q", inv.Script, r.Name),
		}
	}
	return nil
}

// PreviewInvocation implements catalog.InvocationRunner: the body, verbatim.
//
// --preview does not start the plugin. A plugin body is a program, and the
// only faithful answer to "what will this do" without running it is the
// program itself. Anything else would be a guess dressed as a fact, and a
// guess needs its own flag rather than quietly borrowing this one.
func (r Runner) PreviewInvocation(inv catalog.Invocation) ([]string, error) {
	return strings.Split(strings.TrimRight(inv.Body, "\n"), "\n"), nil
}

func nonNil(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// Register loads every plugin a catalog declares and registers the runners
// they provide.
//
// A plugin that will not load stops the run. A catalog that names a plugin has
// already decided it is part of the build; carrying on without it would mean
// silently running something other than what the file says.
func Register(ctx context.Context, rt *Runtime, cat *catalog.Catalog, reg *catalog.RunnerRegistry, root string, stdout, stderr io.Writer, stdin io.Reader) error {
	for _, spec := range cat.Engine.Plugins {
		p, err := rt.Load(ctx, spec.Source, spec.SHA256, root, spec.Provides, spec.Config)
		if err != nil {
			return err
		}
		for _, entry := range spec.Provides {
			kind, name, ok := cutKind(entry)
			if !ok || kind != "runner" {
				// dialect plugins are declared the same way and are not
				// implemented; the catalog already validated the shape.
				continue
			}
			runner := Runner{
				Plugin: p,
				Name:   catalog.RunnerName(name),
				Host:   Host{Dir: root, Stdout: stdout, Stderr: stderr, Stdin: stdin},
			}
			if err := reg.Register(catalog.RunnerName(name), runner); err != nil {
				return fmt.Errorf("plugin %s: %w", spec.Source, err)
			}
		}
	}
	return nil
}

func cutKind(entry string) (kind, name string, ok bool) {
	for i := 0; i < len(entry); i++ {
		if entry[i] == ':' {
			return entry[:i], entry[i+1:], true
		}
	}
	return "", "", false
}
