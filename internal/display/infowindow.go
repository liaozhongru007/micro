package display

import (
	"unicode/utf8"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/zyedidia/micro/v2/internal/buffer"
	"github.com/zyedidia/micro/v2/internal/config"
	"github.com/zyedidia/micro/v2/internal/info"
	"github.com/zyedidia/micro/v2/internal/screen"
	"github.com/zyedidia/micro/v2/internal/util"
	"github.com/zyedidia/tcell"
)

type InfoWindow struct {
	*info.InfoBuf
	*View

	hscroll int
}

func (i *InfoWindow) errStyle() tcell.Style {
	errStyle := config.DefStyle.
		Foreground(tcell.ColorBlack).
		Background(tcell.ColorMaroon)

	if _, ok := config.Colorscheme["error-message"]; ok {
		errStyle = config.Colorscheme["error-message"]
	}

	return errStyle
}

func (i *InfoWindow) defStyle() tcell.Style {
	defStyle := config.DefStyle

	if _, ok := config.Colorscheme["message"]; ok {
		defStyle = config.Colorscheme["message"]
	}

	return defStyle
}

func NewInfoWindow(b *info.InfoBuf) *InfoWindow {
	iw := new(InfoWindow)
	iw.InfoBuf = b
	iw.View = new(View)

	iw.Width, iw.Y = screen.Screen.Size()
	iw.Y--

	return iw
}

func (i *InfoWindow) Resize(w, h int) {
	i.Width = w
	i.Y = h
}

func (i *InfoWindow) SetBuffer(b *buffer.Buffer) {
	i.InfoBuf.Buffer = b
}

func (i *InfoWindow) Relocate() bool   { return false }
func (i *InfoWindow) GetView() *View   { return i.View }
func (i *InfoWindow) SetView(v *View)  {}
func (i *InfoWindow) SetActive(b bool) {}
func (i *InfoWindow) IsActive() bool   { return true }

func (i *InfoWindow) LocFromVisual(vloc buffer.Loc) buffer.Loc {
	c := i.Buffer.GetActiveCursor()
	l := i.Buffer.LineBytes(0)
	n := utf8.RuneCountInString(i.Msg)
	return buffer.Loc{c.GetCharPosInLine(l, vloc.X-n), 0}
}

func (i *InfoWindow) Clear() {
	for x := 0; x < i.Width; x++ {
		screen.SetContent(x, i.Y, ' ', nil, i.defStyle())
	}
}

func (i *InfoWindow) displayBuffer() {
	b := i.Buffer
	line := b.LineBytes(0)
	activeC := b.GetActiveCursor()

	blocX := 0
	vlocX := utf8.RuneCountInString(i.Msg)

	tabsize := 4
	line, nColsBeforeStart, bslice := util.SliceVisualEnd(line, blocX, tabsize)
	blocX = bslice

	draw := func(r rune, style tcell.Style) {
		if nColsBeforeStart <= 0 {
			bloc := buffer.Loc{X: blocX, Y: 0}
			if activeC.HasSelection() &&
				(bloc.GreaterEqual(activeC.CurSelection[0]) && bloc.LessThan(activeC.CurSelection[1]) ||
					bloc.LessThan(activeC.CurSelection[0]) && bloc.GreaterEqual(activeC.CurSelection[1])) {
				// The current character is selected
				style = i.defStyle().Reverse(true)

				if s, ok := config.Colorscheme["selection"]; ok {
					style = s
				}

			}

			rw := runewidth.RuneWidth(r)
			for j := 0; j < rw; j++ {
				c := r
				if j > 0 {
					c = ' '
				}
				screen.SetContent(vlocX, i.Y, c, nil, style)
			}
			vlocX++
		}
		nColsBeforeStart--
	}

	totalwidth := blocX - nColsBeforeStart
	for len(line) > 0 {
		curVX := vlocX
		curBX := blocX
		r, size := utf8.DecodeRune(line)

		draw(r, i.defStyle())

		width := 0

		char := ' '
		switch r {
		case '\t':
			ts := tabsize - (totalwidth % tabsize)
			width = ts
		default:
			width = runewidth.RuneWidth(r)
			char = '@'
		}

		blocX++
		line = line[size:]

		// Draw any extra characters either spaces for tabs or @ for incomplete wide runes
		if width > 1 {
			for j := 1; j < width; j++ {
				draw(char, i.defStyle())
			}
		}
		if activeC.X == curBX {
			screen.ShowCursor(curVX, i.Y)
		}
		totalwidth += width
		if vlocX >= i.Width {
			break
		}
	}
	if activeC.X == blocX {
		screen.ShowCursor(vlocX, i.Y)
	}
}

var keydisplay = []string{"^Q Quit, ^S Save, ^O Open, ^G Help, ^E Command Bar, ^K Cut Line", "^F Find, ^Z Undo, ^Y Redo, ^A Select All, ^D Duplicate Line, ^T New Tab"}

func (i *InfoWindow) displayKeyMenu() {
	// TODO: maybe make this based on the actual keybindings

	for y := 0; y < len(keydisplay); y++ {
		for x := 0; x < i.Width; x++ {
			if x < len(keydisplay[y]) {
				screen.SetContent(x, i.Y-len(keydisplay)+y, rune(keydisplay[y][x]), nil, i.defStyle())
			} else {
				screen.SetContent(x, i.Y-len(keydisplay)+y, ' ', nil, i.defStyle())
			}
		}
	}
}

func (i *InfoWindow) totalSize() int {
	sum := 0
	for _, n := range i.Suggestions {
		sum += runewidth.StringWidth(n) + 1
	}
	return sum
}

func (i *InfoWindow) scrollToSuggestion() {
	x := 0
	s := i.totalSize()

	for j, n := range i.Suggestions {
		c := utf8.RuneCountInString(n)
		if j == i.CurSuggestion {
			if x+c >= i.hscroll+i.Width {
				i.hscroll = util.Clamp(x+c+1-i.Width, 0, s-i.Width)
			} else if x < i.hscroll {
				i.hscroll = util.Clamp(x-1, 0, s-i.Width)
			}
			break
		}
		x += c + 1
	}

	if s-i.Width <= 0 {
		i.hscroll = 0
	}
}

func (i *InfoWindow) Display() {
	if i.HasPrompt || config.GlobalSettings["infobar"].(bool) {
		i.Clear()
		x := 0
		if config.GetGlobalOption("keymenu").(bool) {
			i.displayKeyMenu()
		}

		if !i.HasPrompt && !i.HasMessage && !i.HasError {
			return
		}
		i.Clear()
		style := i.defStyle()

		if i.HasError {
			style = i.errStyle()
		}

		display := i.Msg
		for _, c := range display {
			screen.SetContent(x, i.Y, c, nil, style)
			x += runewidth.RuneWidth(c)
		}

		if i.HasPrompt {
			i.displayBuffer()
		}
	}

	if i.HasSuggestions && len(i.Suggestions) > 1 {
		i.scrollToSuggestion()

		x := -i.hscroll
		done := false

		statusLineStyle := config.DefStyle.Reverse(true)
		if style, ok := config.Colorscheme["statusline"]; ok {
			statusLineStyle = style
		}
		keymenuOffset := 0
		if config.GetGlobalOption("keymenu").(bool) {
			keymenuOffset = len(keydisplay)
		}

		draw := func(r rune, s tcell.Style) {
			y := i.Y - keymenuOffset - 1
			rw := runewidth.RuneWidth(r)
			for j := 0; j < rw; j++ {
				c := r
				if j > 0 {
					c = ' '
				}

				if x == i.Width-1 && !done {
					screen.SetContent(i.Width-1, y, '>', nil, s)
					x++
					break
				} else if x == 0 && i.hscroll > 0 {
					screen.SetContent(0, y, '<', nil, s)
				} else if x >= 0 && x < i.Width {
					screen.SetContent(x, y, c, nil, s)
				}
				x++
			}
		}

		for j, s := range i.Suggestions {
			style := statusLineStyle
			if i.CurSuggestion == j {
				style = style.Reverse(true)
			}
			for _, r := range s {
				draw(r, style)
				// screen.SetContent(x, i.Y-keymenuOffset-1, r, nil, style)
			}
			draw(' ', statusLineStyle)
		}

		for x < i.Width {
			draw(' ', statusLineStyle)
		}
	}
}
