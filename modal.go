package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ModalAction int

const (
	ModalActionNone ModalAction = iota
	ModalActionCommit
	ModalActionStash
	ModalActionCancel
)

// ModalFocus represents the different focusable sections in the modal
type ModalFocus int

const (
	FocusGitStatus   ModalFocus = iota // Git status file list
	FocusSubject                       // Commit subject field
	FocusDescription                   // Commit description field
)

type CommitModal struct {
	visible       bool
	subject       textinput.Model
	description   textarea.Model
	focusIndex    ModalFocus
	action        ModalAction
	targetBranch  string
	gitStatus     []GitFileStatus
	expandedFiles map[string]bool
	selectedFile  int // Index of currently selected file
	gitService    *GitService

	// Key bindings
	keys CommitModalKeyMap
}

type CommitModalKeyMap struct {
	Tab      key.Binding
	ShiftTab key.Binding
	Commit   key.Binding
	Stash    key.Binding
	Cancel   key.Binding
	Up       key.Binding
	Down     key.Binding
}

var commitModalKeys = CommitModalKeyMap{
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next field"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev field"),
	),
	Commit: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "commit & switch"),
	),
	Stash: key.NewBinding(
		key.WithKeys("ctrl+t"),
		key.WithHelp("ctrl+t", "stash & switch"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
}

// Add new key binding for expanding files
var expandKey = key.NewBinding(
	key.WithKeys("space", "enter"),
	key.WithHelp("space/enter", "expand file"),
)

var (
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(1, 2).
			Width(70).
			Height(20)

	modalTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 0, 1, 0)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Bold(true)

	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("39")).
			Padding(0, 2).
			Margin(0, 1)

	buttonActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("205")).
				Padding(0, 2).
				Margin(0, 1).
				Bold(true)

	modalHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Padding(1, 0, 0, 0)

	overlayStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("0"))
)

func NewCommitModal() *CommitModal {
	subject := textinput.New()
	subject.Placeholder = "Brief description of changes"
	subject.Focus()
	subject.CharLimit = 72
	subject.Width = 60

	description := textarea.New()
	description.Placeholder = "Optional detailed description..."
	description.SetWidth(60)
	description.SetHeight(5)

	return &CommitModal{
		subject:       subject,
		description:   description,
		keys:          commitModalKeys,
		focusIndex:    FocusGitStatus,
		expandedFiles: make(map[string]bool),
		gitService:    NewGitService(),
	}
}

func (m *CommitModal) Show(targetBranch string) {
	m.visible = true
	m.targetBranch = targetBranch
	m.action = ModalActionNone
	m.focusIndex = FocusGitStatus // Start focused on git status section
	m.subject.Blur()              // Start with subject unfocused
	m.description.Blur()
	m.subject.SetValue("")
	m.description.SetValue("")

	// Load git status
	if status, err := m.gitService.GetGitStatus(); err == nil {
		m.gitStatus = status
	} else {
		m.gitStatus = []GitFileStatus{}
	}

	// Reset expanded files and selected file
	m.expandedFiles = make(map[string]bool)
	m.selectedFile = 0
}

func (m *CommitModal) Hide() {
	m.visible = false
	m.action = ModalActionNone
	m.subject.Blur()
	m.description.Blur()
}

func (m *CommitModal) IsVisible() bool {
	return m.visible
}

func (m *CommitModal) GetAction() ModalAction {
	return m.action
}

func (m *CommitModal) GetCommitMessage() (string, string) {
	return m.subject.Value(), m.description.Value()
}

func (m *CommitModal) GetTargetBranch() string {
	return m.targetBranch
}

func (m *CommitModal) Update(msg tea.Msg) (*CommitModal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.action = ModalActionCancel
			return m, nil

		case key.Matches(msg, m.keys.Commit):
			if strings.TrimSpace(m.subject.Value()) != "" {
				m.action = ModalActionCommit
				return m, nil
			}

		case key.Matches(msg, m.keys.Stash):
			m.action = ModalActionStash
			return m, nil

		case key.Matches(msg, m.keys.Tab):
			m.nextField()

		case key.Matches(msg, m.keys.ShiftTab):
			m.prevField()

		case key.Matches(msg, m.keys.Up):
			if m.focusIndex == FocusGitStatus { // In git status section
				if len(m.gitStatus) > 0 && m.selectedFile > 0 {
					m.selectedFile--
				}
			} else if m.focusIndex == FocusDescription { // If in description, move to subject
				m.focusIndex = FocusSubject
				m.subject.Focus()
				m.description.Blur()
			}

		case key.Matches(msg, m.keys.Down):
			if m.focusIndex == FocusGitStatus { // In git status section
				if len(m.gitStatus) > 0 && m.selectedFile < len(m.gitStatus)-1 {
					m.selectedFile++
				}
			} else if m.focusIndex == FocusSubject { // If in subject, move to description
				m.focusIndex = FocusDescription
				m.subject.Blur()
				m.description.Focus()
			}

		case msg.String() == " " || msg.String() == "enter":
			// Toggle expansion of selected file (only when focused on git status)
			if m.focusIndex == FocusGitStatus && len(m.gitStatus) > 0 && m.selectedFile < len(m.gitStatus) {
				selectedFilePath := m.gitStatus[m.selectedFile].Path
				m.expandedFiles[selectedFilePath] = !m.expandedFiles[selectedFilePath]
			}

		default:
			// Handle input in the focused field
			if m.focusIndex == FocusSubject { // Subject field
				m.subject, cmd = m.subject.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.focusIndex == FocusDescription { // Description field
				m.description, cmd = m.description.Update(msg)
				cmds = append(cmds, cmd)
			}
			// When focusIndex == FocusGitStatus, don't pass keys to text inputs
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *CommitModal) nextField() {
	m.focusIndex = ModalFocus((int(m.focusIndex) + 1) % 3) // Cycle through all focus states
	m.updateFieldFocus()
}

func (m *CommitModal) prevField() {
	newIndex := int(m.focusIndex) - 1
	if newIndex < 0 {
		newIndex = 2 // FocusDescription
	}
	m.focusIndex = ModalFocus(newIndex)
	m.updateFieldFocus()
}

func (m *CommitModal) updateFieldFocus() {
	switch m.focusIndex {
	case FocusGitStatus: // Git status section
		m.subject.Blur()
		m.description.Blur()
	case FocusSubject: // Subject field
		m.subject.Focus()
		m.description.Blur()
	case FocusDescription: // Description field
		m.subject.Blur()
		m.description.Focus()
	}
}

func (m *CommitModal) View() string {
	if !m.visible {
		return ""
	}

	title := modalTitleStyle.Render(fmt.Sprintf("Uncommitted Changes - Switching to '%s'", m.targetBranch))

	// Git status section
	statusSection := m.renderGitStatus()

	subjectLabel := labelStyle.Render("Commit Subject:")
	subjectInput := m.subject.View()

	descLabel := labelStyle.Render("Description (optional):")
	descInput := m.description.View()

	// Buttons
	commitBtn := buttonStyle.Render("Commit & Switch")
	stashBtn := buttonStyle.Render("Stash & Switch")
	cancelBtn := buttonStyle.Render("Cancel")

	if m.focusIndex == FocusDescription { // If we were to add button focus
		commitBtn = buttonActiveStyle.Render("Commit & Switch")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Left, commitBtn, stashBtn, cancelBtn)

	help := modalHelpStyle.Render("ctrl+s: commit • ctrl+t: stash • space: expand file • esc: cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		statusSection,
		"",
		subjectLabel,
		subjectInput,
		"",
		descLabel,
		descInput,
		"",
		buttons,
		"",
		help,
	)

	modal := modalStyle.Render(content)

	// Center the modal (this is a simple centering approach)
	return lipgloss.Place(80, 25, lipgloss.Center, lipgloss.Center, modal)
}

func (m *CommitModal) renderGitStatus() string {
	if len(m.gitStatus) == 0 {
		return labelStyle.Render("No changes detected")
	}

	filePathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))                                   // Cyan
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")) // Highlighted

	var lines []string
	focusIndicator := ""
	if m.focusIndex == FocusGitStatus {
		focusIndicator = " [FOCUSED - ↑↓ to navigate, space to expand]"
	}
	lines = append(lines, labelStyle.Render("Changed Files:"+focusIndicator))

	for i, file := range m.gitStatus {
		// Status indicator
		var statusIcon, statusColor string
		switch file.Status {
		case "A":
			statusIcon = "+"
			statusColor = "42" // Green
		case "M":
			statusIcon = "~"
			statusColor = "226" // Yellow
		case "D":
			statusIcon = "-"
			statusColor = "196" // Red
		case "R":
			statusIcon = "→"
			statusColor = "39" // Cyan
		default:
			statusIcon = "?"
			statusColor = "241" // Gray
		}

		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)

		// Staged/unstaged indicators
		stagedIndicator := ""
		if file.StagedStatus != " " && file.StagedStatus != "?" {
			stagedIndicator += "S"
		}
		if file.WorkStatus != " " && file.WorkStatus != "?" {
			stagedIndicator += "W"
		}
		if stagedIndicator == "" {
			stagedIndicator = " "
		}

		// File line
		expandIcon := "▶"
		if m.expandedFiles[file.Path] {
			expandIcon = "▼"
		}

		// Selection indicator
		selectionIndicator := " "
		if i == m.selectedFile && m.focusIndex == FocusGitStatus {
			selectionIndicator = ">"
		}

		// Format line count statistics
		var lineStats string
		if file.LinesAdded > 0 || file.LinesDeleted > 0 {
			addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))    // Green
			deletedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red

			if file.LinesAdded > 0 && file.LinesDeleted > 0 {
				lineStats = fmt.Sprintf(" (%s, %s)",
					addedStyle.Render(fmt.Sprintf("+%d", file.LinesAdded)),
					deletedStyle.Render(fmt.Sprintf("-%d", file.LinesDeleted)))
			} else if file.LinesAdded > 0 {
				lineStats = fmt.Sprintf(" (%s)",
					addedStyle.Render(fmt.Sprintf("+%d", file.LinesAdded)))
			} else if file.LinesDeleted > 0 {
				lineStats = fmt.Sprintf(" (%s)",
					deletedStyle.Render(fmt.Sprintf("-%d", file.LinesDeleted)))
			}
		}

		fileLine := fmt.Sprintf(" %s %s %s [%s] %s%s",
			selectionIndicator,
			expandIcon,
			statusStyle.Render(statusIcon),
			stagedIndicator,
			filePathStyle.Render(file.Path),
			lineStats)

		// Highlight selected file
		if i == m.selectedFile && m.focusIndex == FocusGitStatus {
			fileLine = selectedStyle.Render(fileLine)
		}

		lines = append(lines, fileLine)

		// Show diff if expanded
		if m.expandedFiles[file.Path] {
			if diff, err := m.gitService.GetFileDiff(file.Path); err == nil {
				// Truncate diff for display (show first few lines)
				diffLines := strings.Split(diff, "\n")
				maxLines := 5
				if len(diffLines) > maxLines {
					diffLines = diffLines[:maxLines]
					diffLines = append(diffLines, "    ... (truncated)")
				}

				diffStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
				for _, diffLine := range diffLines {
					if strings.TrimSpace(diffLine) != "" {
						lines = append(lines, "    "+diffStyle.Render(diffLine))
					}
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (m *CommitModal) ViewOverlay(content string) string {
	if !m.visible {
		return content
	}

	// Simply return the modal view - this replaces the entire screen when modal is visible
	// This is simpler and more reliable than trying to overlay
	return m.View()
}
