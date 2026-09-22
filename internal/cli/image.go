package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
)

// Flags for tap image add.
var (
	imageAddSlide int
	imageAddJSON  bool
)

// imageCmd groups the commands for a deck's images.
var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Add and generate a deck's images",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUnknownGroupSubcommand,
}

// imageAddCmd copies an image into a deck's images folder.
var imageAddCmd = &cobra.Command{
	Use:   "add <file> [deck]",
	Short: "Copy an image into the deck's images folder",
	Long: `Copy an image into the images/ folder next to a deck and print the
markdown that shows it.

The copy keeps the file name, with spaces, parentheses and other
characters that would need escaping in a markdown link replaced by "-".
When the resulting name is taken, tap adds -2, -3 and so on before the
extension. With --slide N, tap also adds the markdown at the end of
slide N.

[deck] is a deck file or folder. With no deck, tap uses the deck in the
current folder.

Examples:
  tap image add ~/Desktop/diagram.png
  tap image add diagram.png talk.md --slide 3
  tap image add diagram.png --json`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runImageAdd,
}

func init() {
	rootCmd.AddCommand(imageCmd)
	imageCmd.AddCommand(imageAddCmd)

	imageAddCmd.Flags().IntVar(&imageAddSlide, "slide", 0, "also add the image at the end of this slide, from 1")
	imageAddCmd.Flags().BoolVar(&imageAddJSON, "json", false, "print the result as JSON")
}

// addedImageResult is the --json result of tap image add. Slide is 0 when
// --slide was not given.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type addedImageResult struct {
	Deck     string `json:"deck"`
	Image    string `json:"image"`
	Markdown string `json:"markdown"`
	Slide    int    `json:"slide,omitempty"`
}

func runImageAdd(cmd *cobra.Command, args []string) error {
	source := args[0]
	var deckArg string
	if len(args) > 1 {
		deckArg = args[1]
	}
	deck, err := resolveDeck(deckArg)
	if err != nil {
		return err
	}

	hasSlide := cmd.Flags().Changed("slide")
	slideIndex := 0
	if hasSlide {
		if slideIndex, err = slideIndexFromFlag(deck, imageAddSlide); err != nil {
			return err
		}
	}

	added, err := deckedit.AddImage(deck, source)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return userError(codeFileNotFound, fmt.Errorf("file not found: %s", source))
	case errors.Is(err, deckedit.ErrNotAnImage):
		return userError(codeNotAnImage, err)
	case err != nil:
		return internalError(codeInternal, err)
	}

	if hasSlide {
		if err := deckedit.InsertIntoFile(deck, slideIndex, added.Markdown); err != nil {
			return internalError(codeInternal, err)
		}
	}

	if imageAddJSON {
		result := addedImageResult{Deck: deck, Image: filepath.ToSlash(added.Path), Markdown: added.Markdown}
		if hasSlide {
			result.Slide = imageAddSlide
		}
		return printJSONOK(cmd.OutOrStdout(), result)
	}
	fmt.Fprintln(cmd.OutOrStdout(), added.Markdown)
	return nil
}

// slideIndexFromFlag checks a 1-based --slide value against the deck and
// returns the zero-based index that internal/deckedit uses.
func slideIndexFromFlag(deck string, slideNumber int) (int, error) {
	content, err := os.ReadFile(deck)
	if err != nil {
		return 0, userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", deck, err))
	}
	total := len(deckedit.SlideBodies(string(content)))
	if slideNumber < 1 || slideNumber > total {
		return 0, userError(codeOutOfRange, slideOutOfRangeError(slideNumber, total))
	}
	return slideNumber - 1, nil
}
