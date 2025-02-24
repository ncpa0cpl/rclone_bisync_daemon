package main

import (
	"strconv"
	"strings"
)

type ParsedArgs struct {
	Input       string
	NamedParams map[string]string
}

func (args *ParsedArgs) HasParam(paramName string) bool {
	_, ok := args.NamedParams[paramName]
	return ok
}

func (args *ParsedArgs) GetParam(paramName string, defaultValue string) string {
	if v, ok := args.NamedParams[paramName]; ok {
		return v
	}
	return defaultValue
}

func (args *ParsedArgs) GetParamInt(paramName string, defaultValue int) int {
	if v, ok := args.NamedParams[paramName]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func (args *ParsedArgs) GetParamInt64(paramName string, defaultValue int64) int64 {
	if v, ok := args.NamedParams[paramName]; ok {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

func (args *ParsedArgs) GetParamUint64(paramName string, defaultValue uint64) uint64 {
	if v, ok := args.NamedParams[paramName]; ok {
		if i, err := strconv.ParseUint(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

func removePrefix(argName string) string {
	if strings.HasPrefix(argName, "--") {
		return argName[2:]
	}
	return argName[1:]
}

type BoolArgsList []string

func (barg BoolArgsList) isOne(arg string) bool {
	for _, a := range barg {
		if a == arg {
			return true
		}
	}
	return false
}

func ParseArgs(args []string, boolArgs BoolArgsList) ParsedArgs {
	results := ParsedArgs{
		Input:       "",
		NamedParams: make(map[string]string),
	}

	for i := 0; i < len(args); i += 1 {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			vIdx := i + 1
			if !boolArgs.isOne(arg) && vIdx < len(args) && !strings.HasPrefix(args[vIdx], "-") {
				results.NamedParams[removePrefix(arg)] = args[vIdx]
				i += 1
			} else {
				results.NamedParams[removePrefix(arg)] = ""
			}
		} else {
			if results.Input == "" {
				results.Input = arg
			}
		}

	}

	return results
}
