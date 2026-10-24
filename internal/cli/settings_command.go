package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

var (
	settingsGetJSON bool
	settingsSetJSON bool
)

// settingKey is one key tap settings reads and writes: how to read its
// value from the settings, and how to store a new one.
type settingKey struct {
	get func(usersettings.Settings) bool
	set func(*usersettings.Settings, bool)
}

// settingKeys are the keys tap settings knows, by their dotted
// settings.yaml path.
var settingKeys = map[string]settingKey{
	"present.startTimerOnFirstAdvance": {
		get: func(settings usersettings.Settings) bool { return settings.Present.TimerStartsOnFirstAdvance() },
		set: func(settings *usersettings.Settings, value bool) { settings.Present.StartTimerOnFirstAdvance = &value },
	},
}

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Read and change your tap settings",
	Long: `Read and change the settings in ~/.config/tap/settings.yaml, which tap
present, tap dev and the Tap desktop app share.

Keys:
  present.startTimerOnFirstAdvance  hold the presenter timer at 0:00 until
                                    you leave the first slide (default true)`,
}

var settingsGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print a setting",
	Long: `Print a setting's value, or its default when it was never set.

Examples:
  tap settings get present.startTimerOnFirstAdvance
  tap settings get present.startTimerOnFirstAdvance --json`,
	Args: cobra.ExactArgs(1),
	RunE: runSettingsGet,
}

var settingsSetCmd = &cobra.Command{
	Use:   "set <key> <true|false>",
	Short: "Change a setting",
	Long: `Change a setting. A running tap picks it up the next time a page loads.

Examples:
  tap settings set present.startTimerOnFirstAdvance false`,
	Args: cobra.ExactArgs(2),
	RunE: runSettingsSet,
}

func init() {
	rootCmd.AddCommand(settingsCmd)
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)
	settingsGetCmd.Flags().BoolVar(&settingsGetJSON, "json", false, "print the setting as JSON")
	settingsSetCmd.Flags().BoolVar(&settingsSetJSON, "json", false, "print the result as JSON")
}

func lookupSettingKey(name string) (settingKey, error) {
	key, ok := settingKeys[name]
	if !ok {
		names := make([]string, 0, len(settingKeys))
		for known := range settingKeys {
			names = append(names, known)
		}
		slices.Sort(names)
		return settingKey{}, userError(codeUsage, fmt.Errorf("unknown setting %q; known settings: %s", name, strings.Join(names, ", ")))
	}
	return key, nil
}

func printSetting(cmd *cobra.Command, asJSON bool, name string, value bool) error {
	out := cmd.OutOrStdout()
	if asJSON {
		return printJSONOK(out, struct {
			Key   string `json:"key"`
			Value bool   `json:"value"`
		}{Key: name, Value: value})
	}
	fmt.Fprintln(out, value)
	return nil
}

func runSettingsGet(cmd *cobra.Command, args []string) error {
	key, err := lookupSettingKey(args[0])
	if err != nil {
		return err
	}
	_, settings, err := loadApprovalSettings()
	if err != nil {
		return err
	}
	return printSetting(cmd, settingsGetJSON, args[0], key.get(settings))
}

func runSettingsSet(cmd *cobra.Command, args []string) error {
	key, err := lookupSettingKey(args[0])
	if err != nil {
		return err
	}
	value, err := strconv.ParseBool(args[1])
	if err != nil {
		return userError(codeUsage, fmt.Errorf("%s takes true or false, not %q", args[0], args[1]))
	}
	settingsPath, err := usersettings.Path()
	if err != nil {
		return internalError(codeInternal, err)
	}
	lockErr := usersettings.WithLock(settingsPath, func() error {
		settings, err := usersettings.Load(settingsPath)
		if err != nil {
			return userError(codeInvalidSettings, err)
		}
		key.set(&settings, value)
		if err := usersettings.Save(settingsPath, settings); err != nil {
			return internalError(codeInternal, err)
		}
		return nil
	})
	if lockErr != nil {
		return lockErr
	}
	return printSetting(cmd, settingsSetJSON, args[0], value)
}
