package components

import (
	"os"

	"github.com/evanw/esbuild/pkg/api"
)

// assetInlineThreshold is the largest asset esbuild still inlines as a data
// URL. An imported image or font this size or larger is emitted as its own
// file instead (see assetSizePlugin), served or written out separately
// rather than bloating the bundle's JavaScript.
const assetInlineThreshold = 100 * 1024 // 100 KB

// assetExtensionFilter matches every image and font extension this package
// inlines or emits based on size (see assetSizePlugin). esbuild applies one
// loader per extension, not per size, so choosing inline vs. file per
// asset needs a plugin instead of a fixed per-extension loader mapping.
const assetExtensionFilter = `\.(png|jpe?g|gif|svg|webp|woff2)$`

// assetSizePlugin inlines an imported asset under assetInlineThreshold as a
// data URL, and emits one at or above it as its own output file (esbuild's
// "file" loader), named by AssetNames and referenced by a URL under publicPath.
// The emitted file shows up in the build result's OutputFiles alongside the
// bundle's JavaScript and CSS (see Build), for the caller to serve or write
// out next to them.
func assetSizePlugin(publicPath string) api.Plugin {
	return api.Plugin{
		Name: "tap-asset-size",
		Setup: func(build api.PluginBuild) {
			build.OnLoad(api.OnLoadOptions{Filter: assetExtensionFilter}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				info, err := os.Stat(args.Path)
				if err != nil {
					return api.OnLoadResult{}, err
				}
				contents, err := os.ReadFile(args.Path)
				if err != nil {
					return api.OnLoadResult{}, err
				}
				text := string(contents)

				loader := api.LoaderFile
				if info.Size() < assetInlineThreshold {
					loader = api.LoaderDataURL
				}
				return api.OnLoadResult{Contents: &text, Loader: loader}, nil
			})
		},
	}
}

// assetContentType maps an emitted asset's extension to the content type it
// is served with, matching the extensions assetExtensionFilter covers.
func assetContentType(extension string) string {
	switch extension {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".woff2":
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
