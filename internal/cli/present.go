package cli

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

var (
	presentPort     int
	presentNoRecord bool
	presentLAN      bool
)

var presentCmd = &cobra.Command{
	Use:   "present [deck]",
	Short: "Give the talk: serve the deck, open it, and record the run",
	Long: `Serve the deck for a talk or a practice run of it.

Unlike tap dev, tap present does not reload when files change (press r to
reload), opens the slides at launch, and leaves out the keys that edit the
deck. If you opt in the first time you run it, every tap present run is
recorded from launch until you quit, following the projector across HDMI
swaps.

Examples:
  tap present                  # The deck in this folder
  tap present slides.md
  tap present slides.md --no-record   # skip recording for this run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file, err := resolveDeck(firstArg(args))
		if err != nil {
			return err
		}

		settingsPath, err := usersettings.Path()
		if err != nil {
			return err
		}
		record, err := presentRecordingWanted(consentInput{
			SettingsPath: settingsPath,
			In:           os.Stdin,
			Out:          os.Stdout,
			NoRecord:     presentNoRecord,
			Supported:    recorder.Supported(),
			Interactive:  isatty.IsTerminal(os.Stdin.Fd()),
		})
		if err != nil {
			return err
		}

		return runDevServer(serverOptions{
			file:         file,
			port:         presentPort,
			portExplicit: cmd.Flags().Changed("port"),
			present:      true,
			record:       record,
			lan:          presentLAN,
		})
	},
}

func init() {
	rootCmd.AddCommand(presentCmd)

	presentCmd.Flags().IntVarP(&presentPort, "port", "p", 3000, "port for the server")
	presentCmd.Flags().BoolVar(&presentNoRecord, "no-record", false, "do not record this run")
	presentCmd.Flags().BoolVar(&presentLAN, "lan", false, "listen on the local network too, so a phone on the same network can open the presenter view (default: this machine only)")
}
