package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	diffTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	diffSummaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229"))

	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	diffDelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	diffHunkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	diffCtxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))

	diffFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	diffSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	diffHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

// DiffView is a full-screen view showing the diff between two branches:
// a summary line, then collapsed file entries with +/- line counts that
// expand in place to show the file's diff.
type DiffView struct {
	visible       bool
	base          string // git ref name of the base branch
	target        string // git ref name of the branch being compared
	baseOptions   []string
	files         []DiffFile
	diffCache     map[string][]string // rendered diff lines per file path
	expanded      map[string]bool
	selected      int // index of selected file
	scroll        int // first visible content line
	width         int
	height        int
	selectingBase bool
	baseCursor    int
	err           error
	gitService    *GitService
}

func NewDiffView(gitService *GitService) *DiffView {
	return &DiffView{
		width:      100,
		height:     30,
		gitService: gitService,
	}
}

// Show opens the diff view comparing target against base. baseOptions are
// the branches the user can switch the base to.
func (d *DiffView) Show(base, target string, baseOptions []string) {
	d.visible = true
	d.base = base
	d.target = target
	d.baseOptions = baseOptions
	d.selectingBase = false
	d.baseCursor = 0
	d.reload()
}

func (d *DiffView) reload() {
	d.selected = 0
	d.scroll = 0
	d.expanded = make(map[string]bool)
	d.diffCache = make(map[string][]string)
	d.files, d.err = d.gitService.GetDiffStats(d.base, d.target)
}

func (d *DiffView) Hide() {
	d.visible = false
}

func (d *DiffView) IsVisible() bool {
	return d.visible
}

func (d *DiffView) SetSize(width, height int) {
	d.width = width
	d.height = height
}

func (d *DiffView) Update(msg tea.Msg) (*DiffView, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}

	if d.selectingBase {
		switch keyMsg.String() {
		case "esc", "q", "b":
			d.selectingBase = false
		case "up", "k":
			if d.baseCursor > 0 {
				d.baseCursor--
			}
		case "down", "j":
			if d.baseCursor < len(d.baseOptions)-1 {
				d.baseCursor++
			}
		case "enter", " ":
			if d.baseCursor < len(d.baseOptions) {
				d.base = d.baseOptions[d.baseCursor]
				d.selectingBase = false
				d.reload()
			}
		}
		return d, nil
	}

	switch keyMsg.String() {
	case "esc", "q":
		d.Hide()
	case "b":
		if len(d.baseOptions) > 0 {
			d.selectingBase = true
			d.baseCursor = 0
			for i, opt := range d.baseOptions {
				if opt == d.base {
					d.baseCursor = i
					break
				}
			}
		}
	case "up", "k":
		if d.selected > 0 {
			d.selected--
			d.ensureSelectedVisible()
		}
	case "down", "j":
		if d.selected < len(d.files)-1 {
			d.selected++
			d.ensureSelectedVisible()
		}
	case "enter", " ":
		d.toggleSelected()
	case "pgup":
		d.scroll = max(0, d.scroll-d.viewportHeight())
	case "pgdown":
		d.scroll = min(d.maxScroll(), d.scroll+d.viewportHeight())
	}

	return d, nil
}

func (d *DiffView) toggleSelected() {
	if d.selected >= len(d.files) {
		return
	}
	file := d.files[d.selected]
	if d.expanded[file.Path] {
		d.expanded[file.Path] = false
	} else {
		d.expanded[file.Path] = true
		d.loadDiff(file)
	}
	d.ensureSelectedVisible()
}

func (d *DiffView) loadDiff(file DiffFile) {
	if _, ok := d.diffCache[file.Path]; ok {
		return
	}
	if file.Binary {
		d.diffCache[file.Path] = []string{diffCtxStyle.Render("    (binary file)")}
		return
	}

	raw, err := d.gitService.GetDiffForFile(d.base, d.target, file.Path)
	if err != nil {
		d.diffCache[file.Path] = []string{diffDelStyle.Render("    error: " + err.Error())}
		return
	}

	var lines []string
	maxWidth := max(20, d.width-8)
	for _, line := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		// Skip the per-file header noise; hunks carry the context
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		line = truncateString(line, maxWidth)
		var styled string
		switch {
		case strings.HasPrefix(line, "@@"):
			styled = diffHunkStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			styled = diffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			styled = diffDelStyle.Render(line)
		default:
			styled = diffCtxStyle.Render(line)
		}
		lines = append(lines, "    "+styled)
	}

	if len(lines) == 0 {
		lines = []string{diffCtxStyle.Render("    (no textual changes)")}
	}
	d.diffCache[file.Path] = lines
}

// fileLineIndex returns the content-line index of file i's header row
func (d *DiffView) fileLineIndex(i int) int {
	idx := 0
	for j := 0; j < i && j < len(d.files); j++ {
		idx++
		if d.expanded[d.files[j].Path] {
			idx += len(d.diffCache[d.files[j].Path])
		}
	}
	return idx
}

func (d *DiffView) totalLines() int {
	idx := d.fileLineIndex(len(d.files))
	return idx
}

func (d *DiffView) viewportHeight() int {
	// title + summary + blank + blank + help = 5 lines of chrome
	return max(5, d.height-5)
}

func (d *DiffView) maxScroll() int {
	return max(0, d.totalLines()-d.viewportHeight())
}

func (d *DiffView) ensureSelectedVisible() {
	line := d.fileLineIndex(d.selected)
	if line < d.scroll {
		d.scroll = line
	}
	if line >= d.scroll+d.viewportHeight() {
		d.scroll = line - d.viewportHeight() + 1
	}
	d.scroll = min(d.scroll, d.maxScroll())
	d.scroll = max(0, d.scroll)
}

func (d *DiffView) View() string {
	if !d.visible {
		return ""
	}

	if d.selectingBase {
		return d.viewBasePicker()
	}

	title := diffTitleStyle.Render(fmt.Sprintf("Diff: %s...%s", d.base, d.target))

	if d.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			errorStyle.Render(fmt.Sprintf("Error: %v", d.err)),
			"",
			diffHelpStyle.Render("esc/q: back • b: change base"),
		)
	}

	summary := diffSummaryStyle.Render(d.summaryLine())
	help := diffHelpStyle.Render("↑/↓: files • space/enter: expand • b: change base • pgup/pgdn: scroll • esc/q: back")

	var content []string
	if len(d.files) == 0 {
		content = append(content, diffCtxStyle.Render("No differences."))
	} else {
		for i, file := range d.files {
			content = append(content, d.renderFileLine(i, file))
			if d.expanded[file.Path] {
				content = append(content, d.diffCache[file.Path]...)
			}
		}
	}

	// Viewport
	viewH := d.viewportHeight()
	start := min(d.scroll, max(0, len(content)-1))
	end := min(len(content), start+viewH)
	visible := content[start:end]

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		summary,
		"",
		strings.Join(visible, "\n"),
		"",
		help,
	)
}

func (d *DiffView) summaryLine() string {
	var added, deleted int
	for _, f := range d.files {
		added += f.Added
		deleted += f.Deleted
	}
	fileWord := "files"
	if len(d.files) == 1 {
		fileWord = "file"
	}
	return fmt.Sprintf("  %d %s changed, %s %s  (changes on %s since it diverged from %s)",
		len(d.files), fileWord,
		diffAddStyle.Render(fmt.Sprintf("+%d", added)),
		diffDelStyle.Render(fmt.Sprintf("-%d", deleted)),
		d.target, d.base)
}

func (d *DiffView) renderFileLine(i int, file DiffFile) string {
	expandIcon := "▶"
	if d.expanded[file.Path] {
		expandIcon = "▼"
	}

	cursor := " "
	if i == d.selected {
		cursor = ">"
	}

	var stats string
	if file.Binary {
		stats = diffCtxStyle.Render("(binary)")
	} else {
		stats = fmt.Sprintf("%s %s",
			diffAddStyle.Render(fmt.Sprintf("+%d", file.Added)),
			diffDelStyle.Render(fmt.Sprintf("-%d", file.Deleted)))
	}

	line := fmt.Sprintf(" %s %s %s  %s", cursor, expandIcon, diffFileStyle.Render(file.Path), stats)
	if i == d.selected {
		line = diffSelectedStyle.Render(fmt.Sprintf(" %s %s %s  %s", cursor, expandIcon, file.Path, stripAnsiStats(file)))
	}
	return line
}

// stripAnsiStats renders stats without color so the selection highlight
// covers the whole line uniformly
func stripAnsiStats(file DiffFile) string {
	if file.Binary {
		return "(binary)"
	}
	return fmt.Sprintf("+%d -%d", file.Added, file.Deleted)
}

func (d *DiffView) viewBasePicker() string {
	title := diffTitleStyle.Render("Select base branch")
	sub := diffSummaryStyle.Render(fmt.Sprintf("  comparing against: %s", d.target))

	var lines []string
	for i, opt := range d.baseOptions {
		cursor := "  "
		line := opt
		if opt == d.base {
			line += " (current base)"
		}
		if i == d.baseCursor {
			lines = append(lines, diffSelectedStyle.Render("> "+line))
		} else {
			lines = append(lines, cursor+line)
		}
	}

	help := diffHelpStyle.Render("↑/↓: move • enter: select • esc/b: cancel")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		sub,
		"",
		strings.Join(lines, "\n"),
		"",
		help,
	)
}
