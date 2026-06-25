package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

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

// compactVBoxLayout stacks children vertically with a configurable gap between
// them, producing tighter rows than container.NewVBox (which inserts
// theme.Padding() between every child). A negative spacing pulls neighbouring
// rows together so they overlap the labels' built-in vertical padding, which is
// how the details metadata is condensed to fit 720p screens.
type compactVBoxLayout struct {
	spacing float32
}

func (l compactVBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	var visible int
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if min.Width > w {
			w = min.Width
		}
		h += min.Height
		visible++
	}
	if visible > 1 {
		h += l.spacing * float32(visible-1)
	}
	return fyne.NewSize(w, h)
}

func (l compactVBoxLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var y float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + l.spacing
	}
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
type captionValueLayout struct {
	captionWidth float32
}

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
