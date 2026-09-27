package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/gemini"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// Flags for tap image add.
var (
	imageAddSlide int
	imageAddJSON  bool
)

// Flags for tap image generate.
var (
	imageGenerateSlide      int
	imageGeneratePrompt     string
	imageGenerateJSON       bool
	imageGenerateAspect     string
	imageGenerateMatchTheme bool
)

// Flags for tap image regenerate.
var (
	imageRegenerateSlide      int
	imageRegenerateImage      string
	imageRegeneratePrompt     string
	imageRegenerateJSON       bool
	imageRegenerateAspect     string
	imageRegenerateMatchTheme bool
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

// imageGenerateCmd generates an image with AI and adds it to a slide.
var imageGenerateCmd = &cobra.Command{
	Use:   "generate [deck]",
	Short: "Generate an image with AI and add it to a slide",
	Long: `Generate an image from a prompt with Google Gemini, as the i key in
tap dev does, and add it at the end of a slide.

The image is saved as images/generated-<hash>.<ext> next to the deck, and
the slide gets an ai-prompt comment with the prompt, so tap image
regenerate can make it again. GEMINI_API_KEY must be set, in the
environment or in a .env file next to the deck.

--aspect and --match-theme are recorded in the ai-prompt comment and
reused by tap image regenerate. --match-theme puts the deck theme's style
brief (tap theme show --prompt) in front of the prompt sent to the model;
the comment still keeps your words alone.

Examples:
  tap image generate --slide 3 --prompt "a lighthouse at dusk, flat vector"
  tap image generate talk.md --slide 3 --prompt "..." --json
  tap image generate talk.md --slide 3 --prompt "..." --aspect 16:9 --match-theme`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImageGenerate,
}

// imageRegenerateCmd makes an AI image again and replaces it in place.
var imageRegenerateCmd = &cobra.Command{
	Use:   "regenerate [deck]",
	Short: "Generate an AI image again and replace it in place",
	Long: `Generate an AI image on a slide again, as the i key in tap dev does,
and replace it where it is. The old image file is deleted.

--image names the image by the path the slide links to, for example
images/generated-1a2b3c4d.png. Without --prompt, tap reuses the prompt in
the image's ai-prompt comment. --aspect and --match-theme default to what
the comment recorded for that image; give either flag to override it.

Examples:
  tap image regenerate --slide 3 --image images/generated-1a2b3c4d.png
  tap image regenerate talk.md --slide 3 --image images/generated-1a2b3c4d.png --prompt "..."
  tap image regenerate talk.md --slide 3 --image images/generated-1a2b3c4d.png --aspect 1:1`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImageRegenerate,
}

func init() {
	rootCmd.AddCommand(imageCmd)
	imageCmd.AddCommand(imageAddCmd)
	imageCmd.AddCommand(imageGenerateCmd)
	imageCmd.AddCommand(imageRegenerateCmd)

	imageAddCmd.Flags().IntVar(&imageAddSlide, "slide", 0, "also add the image at the end of this slide, from 1")
	imageAddCmd.Flags().BoolVar(&imageAddJSON, "json", false, "print the result as JSON")

	imageGenerateCmd.Flags().IntVar(&imageGenerateSlide, "slide", 0, "slide to add the image to, from 1 (required)")
	imageGenerateCmd.Flags().StringVar(&imageGeneratePrompt, "prompt", "", "what the image shows (required)")
	imageGenerateCmd.Flags().BoolVar(&imageGenerateJSON, "json", false, "print the result as JSON")
	imageGenerateCmd.Flags().StringVar(&imageGenerateAspect, "aspect", "", "the image's aspect ratio: 1:1, 16:9, 9:16, 4:3 or 3:4 (default: the model's choice)")
	imageGenerateCmd.Flags().BoolVar(&imageGenerateMatchTheme, "match-theme", false, "put the deck theme's style brief (tap theme show --prompt) in front of the prompt sent to the model; the ai-prompt comment keeps your words")

	imageRegenerateCmd.Flags().IntVar(&imageRegenerateSlide, "slide", 0, "slide the image is on, from 1 (required)")
	imageRegenerateCmd.Flags().StringVar(&imageRegenerateImage, "image", "", "path of the AI image to replace, as the slide links to it (required)")
	imageRegenerateCmd.Flags().StringVar(&imageRegeneratePrompt, "prompt", "", "a new prompt (default: the image's own prompt)")
	imageRegenerateCmd.Flags().BoolVar(&imageRegenerateJSON, "json", false, "print the result as JSON")
	imageRegenerateCmd.Flags().StringVar(&imageRegenerateAspect, "aspect", "", "the image's aspect ratio: 1:1, 16:9, 9:16, 4:3 or 3:4 (default: the recorded choice, or the model's choice)")
	imageRegenerateCmd.Flags().BoolVar(&imageRegenerateMatchTheme, "match-theme", false, "put the deck theme's style brief in front of the prompt (default: the recorded choice)")
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

// generatedImageResult is the --json result of tap image generate and tap
// image regenerate. Replaced is the image that regenerate replaced.
type generatedImageResult struct {
	Deck     string `json:"deck"`
	Slide    int    `json:"slide"`
	Image    string `json:"image"`
	Prompt   string `json:"prompt"`
	Markdown string `json:"markdown"`
	Replaced string `json:"replaced,omitempty"`
}

func runImageGenerate(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("slide") {
		return userError(codeUsage, errors.New("--slide is required"))
	}
	prompt := strings.TrimSpace(imageGeneratePrompt)
	if prompt == "" {
		return userError(codeUsage, errors.New("--prompt is required"))
	}
	if err := refuseChoiceShapedPrompt(prompt); err != nil {
		return err
	}
	deck, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	slideIndex, err := slideIndexFromFlag(deck, imageGenerateSlide)
	if err != nil {
		return err
	}
	options, err := generateOptionsFrom(imageGenerateAspect, imageGenerateMatchTheme)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	placed, err := generateAndPlace(ctx, deckedit.Placement{DeckPath: deck, SlideIndex: slideIndex, Prompt: prompt}, options)
	if err != nil {
		return err
	}

	if imageGenerateJSON {
		return printJSONOK(cmd.OutOrStdout(), generatedImageResult{
			Deck:     deck,
			Slide:    imageGenerateSlide,
			Image:    filepath.ToSlash(placed.Path),
			Prompt:   prompt,
			Markdown: placed.Markdown,
		})
	}
	fmt.Fprintln(cmd.OutOrStdout(), filepath.ToSlash(placed.Path))
	return nil
}

func runImageRegenerate(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("slide") {
		return userError(codeUsage, errors.New("--slide is required"))
	}
	if imageRegenerateImage == "" {
		return userError(codeUsage, errors.New("--image is required"))
	}
	deck, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	slideIndex, err := slideIndexFromFlag(deck, imageRegenerateSlide)
	if err != nil {
		return err
	}
	replacing, err := findAIImage(deck, slideIndex, imageRegenerateImage)
	if err != nil {
		return err
	}
	prompt := replacing.Prompt
	if cmd.Flags().Changed("prompt") {
		prompt = strings.TrimSpace(imageRegeneratePrompt)
		if prompt == "" {
			return userError(codeUsage, errors.New("--prompt is empty"))
		}
		if err := refuseChoiceShapedPrompt(prompt); err != nil {
			return err
		}
	}
	options := generateOptions{aspect: replacing.Aspect, matchTheme: replacing.MatchTheme}
	if cmd.Flags().Changed("aspect") {
		options.aspect = imageRegenerateAspect
	}
	if cmd.Flags().Changed("match-theme") {
		options.matchTheme = imageRegenerateMatchTheme
	}
	options, err = generateOptionsFrom(options.aspect, options.matchTheme)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	placed, err := generateAndPlace(ctx, deckedit.Placement{DeckPath: deck, SlideIndex: slideIndex, Prompt: prompt, Replacing: &replacing}, options)
	if err != nil {
		return err
	}
	if placed.DeleteError != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", placed.DeleteError)
	}

	if imageRegenerateJSON {
		return printJSONOK(cmd.OutOrStdout(), generatedImageResult{
			Deck:     deck,
			Slide:    imageRegenerateSlide,
			Image:    filepath.ToSlash(placed.Path),
			Prompt:   prompt,
			Markdown: placed.Markdown,
			Replaced: filepath.ToSlash(replacing.ImagePath),
		})
	}
	fmt.Fprintln(cmd.OutOrStdout(), filepath.ToSlash(placed.Path))
	return nil
}

// findAIImage returns the AI image on the slide at slideIndex whose link
// is imagePath. Paths compare after cleaning, so ./images/a.png matches
// images/a.png.
func findAIImage(deck string, slideIndex int, imagePath string) (deckedit.AIImage, error) {
	content, err := os.ReadFile(deck)
	if err != nil {
		return deckedit.AIImage{}, userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", deck, err))
	}
	images := deckedit.ParseAIImages(deckedit.SlideBodies(string(content))[slideIndex])
	paths := make([]string, 0, len(images))
	for _, image := range images {
		if filepath.Clean(image.ImagePath) == filepath.Clean(imagePath) {
			return image, nil
		}
		paths = append(paths, image.ImagePath)
	}
	onSlide := "it has none"
	if len(paths) > 0 {
		onSlide = "its AI images are " + strings.Join(paths, ", ")
	}
	return deckedit.AIImage{}, userError(codeImageNotFound, fmt.Errorf("slide %d has no AI image %s: %s", slideIndex+1, imagePath, onSlide))
}

// refuseChoiceShapedPrompt refuses a prompt whose last words the deck's
// ai-prompt comment would read back as a recorded choice, so the words a
// person typed are the words a later regenerate asks for.
func refuseChoiceShapedPrompt(prompt string) error {
	if deckedit.PromptReadsAsChoices(prompt) {
		return userError(codeUsage, errors.New(`--prompt must not end in " | match-theme" or " | aspect: <ratio>"; use --match-theme or --aspect`))
	}
	return nil
}

// generateOptions is what the person chose beyond the prompt.
type generateOptions struct {
	aspect     string
	matchTheme bool
}

// generateOptionsFrom validates --aspect.
func generateOptionsFrom(aspect string, matchTheme bool) (generateOptions, error) {
	if aspect != "" && !slices.Contains(deckedit.AIImageAspectRatios, aspect) {
		return generateOptions{}, userError(codeUsage, fmt.Errorf("--aspect must be one of %s, got %q", strings.Join(deckedit.AIImageAspectRatios, ", "), aspect))
	}
	return generateOptions{aspect: aspect, matchTheme: matchTheme}, nil
}

// themeBriefForDeck is the deck theme's style brief, the text tap theme
// show --prompt prints, for --match-theme. A deck that names no theme or
// an unknown one uses base, as tap renders it. The brief ends with the
// canvas ("Canvas 16:9, sitting on a ... background"); with --aspect the
// canvas names that aspect instead, so the model is not told two sizes.
func themeBriefForDeck(deck, aspect string) (string, error) {
	slug := "base"
	if cfg, err := config.Load(deck); err == nil && cfg.Theme != "" && themes.IsValid(cfg.Theme) {
		slug = cfg.Theme
	}
	theme, ok := findTheme(slug)
	if !ok {
		return "", internalError(codeInternal, fmt.Errorf("theme %q is not built in", slug))
	}
	tokens, ok := themes.Tokens(slug)
	if !ok {
		return "", internalError(codeInternal, fmt.Errorf("theme %q has no tokens", slug))
	}
	illustration, _ := themes.Illustration(slug)
	brief := buildThemePrompt(theme, tokens, illustration)
	if aspect != "" {
		brief = strings.Replace(brief, "Canvas 16:9", "Canvas "+aspect, 1)
	}
	return brief, nil
}

// generateAndPlace generates an image for placement.Prompt and places it
// in the deck through deckedit.PlaceGeneratedImage, the function the TUI
// i key also uses. deckedit.NewImageGenerator reads GEMINI_API_KEY from a
// .env file next to the deck when the environment has none.
func generateAndPlace(ctx context.Context, placement deckedit.Placement, options generateOptions) (deckedit.PlacedImage, error) {
	generator, err := deckedit.NewImageGenerator(placement.DeckPath)
	if err != nil {
		return deckedit.PlacedImage{}, userError(codeNoAPIKey, fmt.Errorf("cannot start image generation: %w", err))
	}
	// What the model is asked can carry the theme's brief; what the deck
	// records is the person's words, so regenerate reads what they wrote.
	request := placement.Prompt
	if options.matchTheme {
		brief, err := themeBriefForDeck(placement.DeckPath, options.aspect)
		if err != nil {
			return deckedit.PlacedImage{}, err
		}
		request = brief + "\n\nThe image shows: " + placement.Prompt
	}
	image, err := generator.GenerateImageWithAspectRatio(ctx, request, options.aspect)
	if err != nil {
		if ctx.Err() != nil {
			return deckedit.PlacedImage{}, errInterrupted
		}
		return deckedit.PlacedImage{}, imageGenerationError(err)
	}
	placement.Aspect = options.aspect
	placement.MatchTheme = options.matchTheme
	placed, err := deckedit.PlaceGeneratedImage(placement, *image)
	if err != nil {
		// A deck or images folder that cannot be written is a problem the
		// person can fix, exit 1, the same classification tap theme set
		// and tap slide add give it.
		return deckedit.PlacedImage{}, userError(codeInvalidDeck, fmt.Errorf("cannot add the image to %s: %w", placement.DeckPath, err))
	}
	return placed, nil
}

// imageGenerationError classifies a failed generation. A network or
// server failure is a problem in the environment (exit 2); a refused
// prompt, a rate limit or a bad key is one the person can fix (exit 1).
func imageGenerationError(err error) error {
	wrapped := fmt.Errorf("image generation failed: %w", err)
	var apiError *gemini.APIError
	if errors.As(err, &apiError) && (apiError.Type == gemini.ErrorTypeNetwork || apiError.Type == gemini.ErrorTypeServer) {
		return internalError(codeImageGeneration, wrapped)
	}
	return userError(codeImageGeneration, wrapped)
}
