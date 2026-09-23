package config

import (
	"fmt"
	"regexp"
	"strings"
)

// variableNamePattern matches an environment variable name.
var variableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// UnsetVariableError is a ${NAME} in a driver setting whose variable is
// not set. A block that uses the setting fails with this message.
type UnsetVariableError struct {
	// Name is the variable, without "${" and "}".
	Name string
	// Setting is where the reference is, such as
	// drivers.mysql.connections.local.password.
	Setting string
}

func (e *UnsetVariableError) Error() string {
	return fmt.Sprintf("%s is not set: %s uses ${%s}. Set it in the environment or in a .env file next to the deck.", e.Name, e.Setting, e.Name)
}

// ExpandEnv replaces each ${NAME} in value with the variable from lookup.
// "$${" writes a literal "${". Any other "$" stays as it is. A variable
// that is not set is an error, never an empty string. setting names where
// value comes from, for the error message.
func ExpandEnv(value, setting string, lookup func(string) (string, bool)) (string, error) {
	var expanded strings.Builder
	for index := 0; index < len(value); {
		rest := value[index:]
		switch {
		case strings.HasPrefix(rest, "$${"):
			expanded.WriteString("${")
			index += len("$${")
		case strings.HasPrefix(rest, "${"):
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return "", fmt.Errorf(`%s has a "${" with no closing "}": write "$${" for a literal "${"`, setting)
			}
			name := rest[len("${"):end]
			if !variableNamePattern.MatchString(name) {
				return "", fmt.Errorf(`%s: %q is not a variable name: write "$${" for a literal "${"`, setting, name)
			}
			resolved, found := lookup(name)
			if !found {
				return "", &UnsetVariableError{Name: name, Setting: setting}
			}
			expanded.WriteString(resolved)
			index += end + 1
		default:
			expanded.WriteByte(value[index])
			index++
		}
	}
	return expanded.String(), nil
}

// Expanded returns the connection with ${NAME} expanded in every string
// value. setting is the connection's place in the frontmatter, such as
// drivers.mysql.connections.local.
func (c ConnectionConfig) Expanded(setting string, lookup func(string) (string, bool)) (ConnectionConfig, error) {
	expanded := c
	for _, field := range []struct {
		value *string
		name  string
	}{
		{&expanded.Host, "host"},
		{&expanded.User, "user"},
		{&expanded.Password, "password"},
		{&expanded.Database, "database"},
		{&expanded.Path, "path"},
	} {
		value, err := ExpandEnv(*field.value, setting+"."+field.name, lookup)
		if err != nil {
			return ConnectionConfig{}, err
		}
		*field.value = value
	}
	return expanded, nil
}

// ExpandedCommand returns a custom driver's command and arguments with
// ${NAME} expanded.
func (d DriverConfig) ExpandedCommand(driverName string, lookup func(string) (string, bool)) (string, []string, error) {
	prefix := "drivers." + driverName
	command, err := ExpandEnv(d.Command, prefix+".command", lookup)
	if err != nil {
		return "", nil, err
	}
	var args []string
	for index, arg := range d.Args {
		expanded, err := ExpandEnv(arg, fmt.Sprintf("%s.args[%d]", prefix, index), lookup)
		if err != nil {
			return "", nil, err
		}
		args = append(args, expanded)
	}
	return command, args, nil
}
