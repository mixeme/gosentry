package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// captionColumnWidth returns the width to reserve for a column of bold
// captions: the widest of them, measured under the current theme so it tracks
// text size and DPI instead of a hand-tuned constant.
func captionColumnWidth(captions ...string) float32 {
	var width float32
	for _, caption := range captions {
		if w := widget.NewLabelWithStyle(caption, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}).MinSize().Width; w > width {
			width = w
		}
	}
	return width
}

type minWidthLayout struct {
	width float32
}

func (l minWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := l.width
	var height float32
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		min := object.MinSize()
		if min.Width > width {
			width = min.Width
		}
		if min.Height > height {
			height = min.Height
		}
	}
	return fyne.NewSize(width, height)
}

func (l minWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		object.Move(fyne.NewPos(0, 0))
		object.Resize(size)
	}
}

// rowOverlap is the (negative) gap that pulls stacked label rows together by
// exactly one label's vertical inner padding. Two adjacent labels each inset
// their text by theme.InnerPadding(), so the whitespace between two lines of
// text is double what a single row needs; removing one label's worth
// condenses the block without letting the text lines touch. Derived rather
// than hard-coded so it follows a theme that changes SizeNameInnerPadding.
func rowOverlap() float32 { return -theme.InnerPadding() }

// initialSplitOffset returns the container.Split offset that opens a horizontal
// split with its leading pane at the given natural width. SetOffset takes a
// ratio, but a pane's natural width is absolute, so the ratio is derived from
// the window width the app opens at rather than written as a literal: 0.44 fits
// 1024 px but would hand a 448 px sidebar 700 px on a 1600 px-wide window.
//
// The divider sits between the panes and is excluded from the ratio, matching
// container.Split's own arithmetic (its divider is two theme paddings thick).
// Split clamps the offset to both panes' minimums when it lays out, so a result
// that is slightly off — the window is a little wider than its content area —
// costs at most a few pixels and can never clip either pane.
func initialSplitOffset(leadingWidth float32) float64 {
	available := float64(defaultWindowWidth - 2*theme.Padding())
	if available <= 0 {
		return 0
	}
	return float64(leadingWidth) / available
}

// fixedHeightLayout forces its contents to a fixed height while leaving the
// width to the parent container. It is used to reserve a stable amount of space
// for the activity panel so a neighbouring widget can absorb the rest.
type fixedHeightLayout struct {
	height float32
}

func (l fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width float32
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		if min := object.MinSize(); min.Width > width {
			width = min.Width
		}
	}
	return fyne.NewSize(width, l.height)
}

func (l fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		object.Move(fyne.NewPos(0, 0))
		object.Resize(fyne.NewSize(size.Width, l.height))
	}
}

// captionValueLayout places a fixed-width caption on the left and lets the value
// fill the remaining width, separated by one theme padding. Capping the caption
// stops it from growing with the window (as an even two-column grid would), so
// the extra space a wider window provides goes entirely to the value column. It
// expects exactly two children: caption first, value second.
//
// The value column has no minimum of its own: it gets whatever the container's
// width leaves, down to zero. What keeps it readable in the details pane is the
// 460 px minimum on commandOutputScroll (jobs_view_details.go), which is that
// pane's widest minimum and therefore its floor — a constant that exists for an
// unrelated reason. A new caller that gives this layout less width gets a value
// column that silently renders narrow or empty rather than one that clips.
type captionValueLayout struct {
	captionWidth float32
}

// MinSize and Layout both return silently when given anything but two
// objects. That is acceptable here because the type is package-private with a
// single constructor (detailRow), which always supplies exactly a caption and
// a value — there is no external caller that could pass the wrong count.
func (l captionValueLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 2 {
		return fyne.Size{}
	}
	captionMin, valueMin := objects[0].MinSize(), objects[1].MinSize()
	height := captionMin.Height
	if valueMin.Height > height {
		height = valueMin.Height
	}
	return fyne.NewSize(l.captionWidth+theme.Padding()+valueMin.Width, height)
}

func (l captionValueLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	caption, value := objects[0], objects[1]
	captionWidth := l.captionWidth
	if captionWidth > size.Width {
		captionWidth = size.Width
	}
	caption.Move(fyne.NewPos(0, 0))
	caption.Resize(fyne.NewSize(captionWidth, size.Height))

	valueX := captionWidth + theme.Padding()
	valueWidth := size.Width - valueX
	if valueWidth < 0 {
		valueWidth = 0
	}
	value.Move(fyne.NewPos(valueX, 0))
	value.Resize(fyne.NewSize(valueWidth, size.Height))
}
