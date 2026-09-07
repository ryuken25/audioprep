package ui

import (
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// LogPanel is the collapsible monospace log. ffmpeg is chatty, so lines are
// buffered and flushed to the widget in batches from the UI thread; writing
// to a Fyne widget on every stderr line would bog the encode down.
type LogPanel struct {
	accordion *widget.Accordion
	grid      *widget.TextGrid
	scroll    *container.Scroll

	mu      sync.Mutex
	lines   []string
	pending bool
}

const maxLogLines = 2000

func NewLogPanel(onCopy func(text string)) *LogPanel {
	l := &LogPanel{}
	l.grid = widget.NewTextGrid()
	l.grid.ShowLineNumbers = false
	l.scroll = container.NewVScroll(l.grid)
	l.scroll.SetMinSize(fyne.NewSize(0, 220))

	copyBtn := widget.NewButtonWithIcon("Copy log", theme.ContentCopyIcon(), func() { onCopy(l.Text()) })
	clearBtn := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), l.Clear)
	bar := container.NewHBox(copyBtn, clearBtn)

	body := container.NewBorder(bar, nil, nil, nil, l.scroll)
	l.accordion = widget.NewAccordion(widget.NewAccordionItem("Log (ffmpeg commands and output)", body))
	return l
}

func (l *LogPanel) Widget() fyne.CanvasObject { return l.accordion }

// Append may be called from any goroutine.
func (l *LogPanel) Append(line string) {
	l.mu.Lock()
	l.lines = append(l.lines, line)
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-maxLogLines:]
	}
	schedule := !l.pending
	l.pending = true
	l.mu.Unlock()

	if schedule {
		// fyne.Do hands the closure to the UI thread. Coalescing: only one
		// flush is queued at a time no matter how many lines arrive.
		fyne.Do(l.flush)
	}
}

func (l *LogPanel) flush() {
	l.mu.Lock()
	text := strings.Join(l.lines, "\n")
	l.pending = false
	l.mu.Unlock()

	l.grid.SetText(text)
	l.scroll.ScrollToBottom()
}

func (l *LogPanel) Clear() {
	l.mu.Lock()
	l.lines = nil
	l.mu.Unlock()
	l.grid.SetText("")
}

func (l *LogPanel) Text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}
