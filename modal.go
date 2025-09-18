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

type CommitModal struct {
	visible      bool
	subject      textinput.Model
	description  textarea.Model
	focusIndex   int
	action       ModalAction
	targetBranch string

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
		subject:     subject,
		description: description,
		keys:        commitModalKeys,
		focusIndex:  0,
	}
}

func (m *CommitModal) Show(targetBranch string) {
	m.visible = true
	m.targetBranch = targetBranch
	m.action = ModalActionNone
	m.focusIndex = 0
	m.subject.Focus()
	m.description.Blur()
	m.subject.SetValue("")
	m.description.SetValue("")
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
			if m.focusIndex == 1 { // If in description, move to subject
				m.focusIndex = 0
				m.subject.Focus()
				m.description.Blur()
			}

		case key.Matches(msg, m.keys.Down):
			if m.focusIndex == 0 { // If in subject, move to description
				m.focusIndex = 1
				m.subject.Blur()
				m.description.Focus()
			}

		default:
			// Handle input in the focused field
			if m.focusIndex == 0 {
				m.subject, cmd = m.subject.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				m.description, cmd = m.description.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *CommitModal) nextField() {
	m.focusIndex = (m.focusIndex + 1) % 2
	if m.focusIndex == 0 {
		m.subject.Focus()
		m.description.Blur()
	} else {
		m.subject.Blur()
		m.description.Focus()
	}
}

func (m *CommitModal) prevField() {
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = 1
	}
	if m.focusIndex == 0 {
		m.subject.Focus()
		m.description.Blur()
	} else {
		m.subject.Blur()
		m.description.Focus()
	}
}

func (m *CommitModal) View() string {
	if !m.visible {
		return ""
	}

	title := modalTitleStyle.Render(fmt.Sprintf("Uncommitted Changes - Switching to '%s'", m.targetBranch))

	subjectLabel := labelStyle.Render("Commit Subject:")
	subjectInput := m.subject.View()

	descLabel := labelStyle.Render("Description (optional):")
	descInput := m.description.View()

	// Buttons
	commitBtn := buttonStyle.Render("Commit & Switch")
	stashBtn := buttonStyle.Render("Stash & Switch")
	cancelBtn := buttonStyle.Render("Cancel")

	if m.focusIndex == 2 { // If we were to add button focus
		commitBtn = buttonActiveStyle.Render("Commit & Switch")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Left, commitBtn, stashBtn, cancelBtn)

	help := modalHelpStyle.Render("ctrl+s: commit • ctrl+t: stash • tab/shift+tab: navigate • esc: cancel")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
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

func (m *CommitModal) ViewOverlay(content string) string {
	if !m.visible {
		return content
	}

	// Create a dim overlay effect
	lines := strings.Split(content, "\n")
	dimmedLines := make([]string, len(lines))

	for i, line := range lines {
		// Dim the background content
		dimmedLines[i] = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(line)
	}

	dimmedContent := strings.Join(dimmedLines, "\n")

	// Overlay the modal on top
	return lipgloss.Place(
		80, 25,
		lipgloss.Center, lipgloss.Center,
		m.View(),
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("240")),
	) + dimmedContent
}
