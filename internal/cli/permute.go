package cli

import (
	"flag"
	"fmt"
	"strings"
)

// Permute reorders args so that flags precede operands, so the standard flag
// package sees every flag regardless of where the user wrote it.
func Permute(fs *flag.FlagSet, args []string) []string {
	var flags []string
	var operands []string
	sawSeparator := false

	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			sawSeparator = true
			operands = append(operands, args[i+1:]...)
			break
		}

		if !isFlagToken(a) {
			operands = append(operands, a)
			continue
		}

		name, hasEq := flagNameAndEq(a)
		if hasEq {
			flags = append(flags, a)
			continue
		}

		f := fs.Lookup(name)
		if f == nil {
			flags = append(flags, a)
			continue
		}

		if isBoolFlag(f) {
			flags = append(flags, a)
			continue
		}

		if i+1 >= len(args) || args[i+1] == "--" {
			operands = append(operands, a)
			continue
		}

		i++
		flags = append(flags, a, args[i])
	}

	if !sawSeparator {
		return append(flags, operands...)
	}

	result := append(flags, "--")
	return append(result, operands...)
}

// SplitCommand finds the subcommand name in args, skipping flags and their values.
func SplitCommand(global *flag.FlagSet, args []string) (name string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			break
		}

		if !isFlagToken(a) {
			rest = append(append([]string{}, args[:i]...), args[i+1:]...)
			return a, rest, nil
		}

		fname, hasEq := flagNameAndEq(a)
		if hasEq {
			continue
		}

		f := global.Lookup(fname)
		if f == nil || isBoolFlag(f) {
			continue
		}

		if i+1 < len(args) && args[i+1] != "--" {
			i++
		}
	}

	return "", nil, fmt.Errorf("cli: no subcommand name found in %v", args)
}

func isFlagToken(a string) bool {
	return len(a) > 1 && a[0] == '-' && a != "--"
}

func flagNameAndEq(a string) (name string, hasEq bool) {
	s := a
	if strings.HasPrefix(s, "--") {
		s = s[2:]
	} else {
		s = s[1:]
	}
	if i := strings.IndexByte(s, '='); i >= 0 {
		return s[:i], true
	}
	return s, false
}

func isBoolFlag(f *flag.Flag) bool {
	bf, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
}
