package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			Padding(0, 1)
)

type model struct {
	tableManager  *TableManager
	gitService    *GitService
	commitModal   *CommitModal
	branches      []Branch
	err           error
	count         int
	message       string
	quitting      bool
	includeRemote bool
	authors       []string
}

func debugLog(msg string, args ...interface{}) {
	if f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(f, msg+"\n", args...)
		f.Close()
	}
}

func main() {
	var (
		count         = flag.Int("n", 10, "Number of branches to show")
		includeRemote = flag.Bool("remote", false, "Include remote branches")
		authorFlag    = flag.String("author", "", "Filter by author(s). Use 'mine' for your commits, 'all' for everyone, or comma-separated usernames")
	)
	flag.Parse()

	// Parse authors
	var authors []string
	if *authorFlag != "" {
		if *authorFlag == "mine" {
			// Will be handled in git service to get current user
			authors = []string{"mine"}
		} else if *authorFlag == "all" {
			authors = []string{"all"}
		} else {
			// Split comma-separated list
			authors = strings.Split(*authorFlag, ",")
			for i, author := range authors {
				authors[i] = strings.TrimSpace(author)
			}
		}
	} else {
		// Default to "mine" when no author flag is provided
		authors = []string{"mine"}
	}

	m := model{
		count:         *count,
		includeRemote: *includeRemote,
		authors:       authors,
		tableManager:  NewTableManager(),
		gitService:    NewGitService(),
		commitModal:   NewCommitModal(),
	}

	if err := m.loadBranches(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m.setupTable()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func (m *model) loadBranches() error {
	branches, err := m.gitService.GetRecentBranches(m.count, m.includeRemote, m.authors)
	if err != nil {
		return err
	}
	m.branches = branches
	return nil
}

func (m *model) setupTable() {
	m.tableManager.SetupTable(m.branches)
}

func (m *model) switchToBranch(branchName string) error {
	// Check for uncommitted changes first
	hasChanges, err := m.gitService.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}

	if hasChanges {
		// Show modal instead of switching immediately
		m.commitModal.Show(branchName)
		return nil
	}

	// No uncommitted changes, switch directly
	return m.gitService.SwitchToBranch(branchName)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			// Get selected branch and switch to it
			if len(m.branches) > 0 {
				selectedRow := m.tableManager.GetCursor()
				if selectedRow < len(m.branches) {
					branchName := m.branches[selectedRow].Name
					if err := m.switchToBranch(branchName); err != nil {
						m.message = fmt.Sprintf("Error: %v", err)
					} else {
						m.message = fmt.Sprintf("Switched to branch: %s", branchName)
						// Refresh branches after switching - this will move the selected branch to top
						if err := m.loadBranches(); err != nil {
							m.err = err
						} else {
							m.setupTable()
						}
					}
				}
			}
			return m, nil
		case "r":
			// Refresh branches
			m.message = "Refreshing..."
			if err := m.loadBranches(); err != nil {
				m.err = err
			} else {
				m.setupTable()
				m.message = "Refreshed!"
			}
			return m, nil
		case "c":
			// Clear message
			m.message = ""
			return m, nil
		}
	}

	// Update table
	table, tableCmd := m.tableManager.UpdateTable(msg)
	m.tableManager.table = table
	cmd = tableCmd

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	var titleText string
	if len(m.authors) > 0 && m.authors[0] != "all" {
		authorText := strings.Join(m.authors, ", ")
		if authorText == "mine" {
			authorText = "my"
		}
		titleText = fmt.Sprintf("Recent Git Branches (%s, %s branches)", authorText, getRemoteText(m.includeRemote))
	} else {
		titleText = fmt.Sprintf("Recent Git Branches (%s)", getRemoteText(m.includeRemote))
	}

	title := titleStyle.Render(titleText)
	help := helpStyle.Render("↑/↓: navigate • enter: switch branch • r: refresh • c: clear message • q: quit")

	var messageView string
	if m.message != "" {
		messageView = successStyle.Render(m.message)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		m.tableManager.View(),
		"",
		messageView,
		help,
	)

	return content
}

func getRemoteText(includeRemote bool) string {
	if includeRemote {
		return "local + remote"
	}
	return "local only"
}
