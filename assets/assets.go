package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed pysentry-icon.png
var iconBytes []byte

func Icon() fyne.Resource {
	return fyne.NewStaticResource("pysentry-icon.png", iconBytes)
}
