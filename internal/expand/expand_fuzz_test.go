package expand_test

import (
	"testing"

	"github.com/my-rv/godo/internal/expand"
)

func FuzzExpand(f *testing.F) {
	f.Add("echo hi", "a", "b")
	f.Add("echo ${godo:argv[name]}", "x", "")
	f.Add("echo ${godo:args}", "one", "two")
	f.Add("echo ${godo:args[0]}", "z", "")
	f.Add("echo ${godo:args[0..2]}", "a", "b")
	f.Add("echo ${godo:argv[bad-name]}", "", "")
	f.Add("echo ${godo:unknown}", "", "")
	f.Add("echo ${godo:args:raw}", "a; rm -rf /", "")
	f.Add("echo ${HOME} ${VAR:-d}", "", "")
	f.Add("echo ${godo:unterminated", "", "")
	f.Add("echo $$", "'", "\\")
	f.Add("echo ${godo:argv[0]}", "", "")
	f.Fuzz(func(t *testing.T, template, a, b string) {
		args := []string{}
		if a != "" {
			args = append(args, a)
		}
		if b != "" {
			args = append(args, b)
		}
		captures := map[string]string{"name": "ok", "x": "1"}
		_, _ = expand.Expand(template, captures, args)
	})
}
