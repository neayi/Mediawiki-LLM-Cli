package main

import (
	"fmt"
	"strings"
)

// args holds the parsed command line of a subcommand. Options may appear
// anywhere among the positional arguments, as --name=value or --name value.
type args struct {
	pos   []string
	vals  map[string]string
	bools map[string]bool
}

func parseArgs(raw []string, valueFlags, boolFlags []string) (*args, error) {
	a := &args{vals: map[string]string{}, bools: map[string]bool{}}
	for i := 0; i < len(raw); i++ {
		arg := normalizeDashes(raw[i])
		if arg == "--" {
			a.pos = append(a.pos, raw[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") {
			a.pos = append(a.pos, arg)
			continue
		}
		name, value, hasValue := strings.Cut(arg[2:], "=")
		switch {
		case contains(boolFlags, name):
			if hasValue {
				return nil, fmt.Errorf("option --%s does not take a value", name)
			}
			a.bools[name] = true
		case contains(valueFlags, name):
			if !hasValue {
				i++
				if i >= len(raw) {
					return nil, fmt.Errorf("option --%s requires a value", name)
				}
				value = raw[i]
			}
			a.vals[name] = value
		default:
			return nil, fmt.Errorf("unknown option --%s", name)
		}
	}
	return a, nil
}

// normalizeDashes turns a leading en dash or em dash into "--", since word
// processors often replace "--" with a typographic dash in copied commands.
func normalizeDashes(s string) string {
	for _, d := range []string{"–", "—"} {
		if rest, ok := strings.CutPrefix(s, d); ok {
			return "--" + rest
		}
	}
	return s
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
