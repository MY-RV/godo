package catalog

import (
	"fmt"
	"strings"
)

// PackageDialect matches literal script keys (first token = name; rest = args).
type PackageDialect struct{}

// Name implements Dialect.
func (PackageDialect) Name() DialectName { return DialectPackage }

// Match implements Dialect. Engine selects the script first; this matches by literal key.
func (PackageDialect) Match(scripts []Script, tokens []string) (*Match, error) {
	if len(tokens) == 0 {
		return nil, ErrNoTokens
	}
	name := tokens[0]
	args := append([]string(nil), tokens[1:]...)
	for _, s := range scripts {
		if s.Key != name {
			continue
		}
		return &Match{
			Script:   s,
			Captures: map[string]string{},
			Args:     args,
			Dialect:  DialectPackage,
		}, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrNoMatch, name)
}

// MatcherDialect matches Express-style patterns; first match in definition order.
type MatcherDialect struct{}

// Name implements Dialect.
func (MatcherDialect) Name() DialectName { return DialectMatcher }

// Match implements Dialect.
func (MatcherDialect) Match(scripts []Script, tokens []string) (*Match, error) {
	if len(tokens) == 0 {
		return nil, ErrNoTokens
	}
	for _, s := range scripts {
		m, ok, err := matchPattern(s.Key, tokens)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		return &Match{
			Script:   s,
			Captures: m.captures,
			Args:     m.args,
			Dialect:  DialectMatcher,
		}, nil
	}
	return nil, fmt.Errorf("%w: %v", ErrNoMatch, tokens)
}

type patMatch struct {
	captures map[string]string
	args     []string
}

func matchPattern(pattern string, tokens []string) (patMatch, bool, error) {
	parts := strings.Fields(pattern)
	if len(parts) == 0 {
		return patMatch{}, false, nil
	}
	if len(tokens) < len(parts) {
		return patMatch{}, false, nil
	}
	caps := make(map[string]string)
	for i, part := range parts {
		tok := tokens[i]
		name, isCap, err := parseCaptureToken(part)
		if err != nil {
			return patMatch{}, false, err
		}
		if isCap {
			caps[name] = tok
			continue
		}
		if part != tok {
			return patMatch{}, false, nil
		}
	}
	return patMatch{captures: caps, args: append([]string(nil), tokens[len(parts):]...)}, true, nil
}

func parseCaptureToken(part string) (name string, ok bool, err error) {
	if !strings.HasPrefix(part, "${") || !strings.HasSuffix(part, "}") {
		return "", false, nil
	}
	if strings.HasPrefix(part, "${godo:") {
		return "", false, nil
	}
	inner := part[2 : len(part)-1]
	if !IsCaptureName(inner) {
		return "", false, fmt.Errorf("%w: %q", ErrInvalidCapture, inner)
	}
	return inner, true, nil
}
