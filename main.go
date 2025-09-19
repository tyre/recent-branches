package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogLevel represents different types of log messages
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	ERROR
	SUCCESS
)

// LogEntry represents a single log entry with level and timestamp
type LogEntry struct {
	Level     LogLevel
	Message   string
	Timestamp time.Time
}

// LogViewer manages the scrollable log display
type LogViewer struct {
	entries      []LogEntry
	scrollOffset int
	maxVisible   int
	maxEntries   int
	focused      bool
	autoScroll   bool
}

func NewLogViewer() *LogViewer {
	return &LogViewer{
		entries:      make([]LogEntry, 0),
		scrollOffset: 0,
		maxVisible:   8,
		maxEntries:   50,
		focused:      false,
		autoScroll:   true,
	}
}

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

	// Log styles
	logContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1).
				Height(8)

	logTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	debugStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	logSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	logFocusedStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(0, 1).
			Height(8)

	// Commit preview styles
	commitContainerStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1).
				Height(6)

	commitTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)

	commitHashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	commitAuthorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226"))

	commitTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type model struct {
	tableManager    *TableManager
	gitService      *GitService
	commitModal     *CommitModal
	logViewer       *LogViewer
	branches        []Branch
	selectedCommits []Commit
	err             error
	count           int
	message         string
	quitting        bool
	includeRemote   bool
	authors         []string
	logs            []string // Keep for backward compatibility
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
		count:           *count,
		includeRemote:   *includeRemote,
		authors:         authors,
		tableManager:    NewTableManager(),
		gitService:      NewGitService(),
		commitModal:     NewCommitModal(),
		logViewer:       NewLogViewer(),
		selectedCommits: []Commit{},
	}

	// Add initial startup logging
	m.logInfo("Application started - Recent Branches v1.0")
	m.logDebug("Configuration: count=%d, includeRemote=%v, authors=%v", m.count, m.includeRemote, m.authors)

	if err := m.loadBranches(); err != nil {
		m.logError("Failed to load branches: %v", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m.logSuccess("Successfully loaded %d branches", len(m.branches))
	m.setupTable()
	m.logDebug("Table setup complete")

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func (m *model) addLog(msg string, args ...interface{}) {
	logMsg := fmt.Sprintf(msg, args...)
	m.logs = append(m.logs, logMsg)
	// Keep only last 10 log messages
	if len(m.logs) > 10 {
		m.logs = m.logs[1:]
	}
}

// Enhanced logging methods
func (m *model) logDebug(msg string, args ...interface{}) {
	m.addLogEntry(DEBUG, msg, args...)
}

func (m *model) logInfo(msg string, args ...interface{}) {
	m.addLogEntry(INFO, msg, args...)
}

func (m *model) logError(msg string, args ...interface{}) {
	m.addLogEntry(ERROR, msg, args...)
}

func (m *model) logSuccess(msg string, args ...interface{}) {
	m.addLogEntry(SUCCESS, msg, args...)
}

func (m *model) addLogEntry(level LogLevel, msg string, args ...interface{}) {
	entry := LogEntry{
		Level:     level,
		Message:   fmt.Sprintf(msg, args...),
		Timestamp: time.Now(),
	}

	m.logViewer.entries = append(m.logViewer.entries, entry)

	// Keep only maxEntries
	if len(m.logViewer.entries) > m.logViewer.maxEntries {
		m.logViewer.entries = m.logViewer.entries[1:]
		if m.logViewer.scrollOffset > 0 {
			m.logViewer.scrollOffset--
		}
	}

	// Auto-scroll to bottom if enabled
	if m.logViewer.autoScroll {
		maxScroll := len(m.logViewer.entries) - m.logViewer.maxVisible
		if maxScroll > 0 {
			m.logViewer.scrollOffset = maxScroll
		}
	}
}

func (m *model) clearLogs() {
	m.logViewer.entries = make([]LogEntry, 0)
	m.logViewer.scrollOffset = 0
	m.logViewer.autoScroll = true
}

// LogViewer methods
func (lv *LogViewer) ScrollUp() {
	if lv.scrollOffset > 0 {
		lv.scrollOffset--
		lv.autoScroll = false
	}
}

func (lv *LogViewer) ScrollDown() {
	maxScroll := len(lv.entries) - lv.maxVisible
	if maxScroll > 0 && lv.scrollOffset < maxScroll {
		lv.scrollOffset++
		// Check if we're at the bottom to re-enable auto-scroll
		if lv.scrollOffset == maxScroll {
			lv.autoScroll = true
		}
	}
}

func (lv *LogViewer) ToggleFocus() {
	lv.focused = !lv.focused
}

func (lv *LogViewer) View() string {
	if len(lv.entries) == 0 {
		emptyMsg := "No debug logs yet..."
		style := logContainerStyle
		if lv.focused {
			style = logFocusedStyle
		}
		return style.Render(timestampStyle.Render(emptyMsg))
	}

	// Calculate visible entries
	start := lv.scrollOffset
	end := start + lv.maxVisible
	if end > len(lv.entries) {
		end = len(lv.entries)
	}
	if start >= len(lv.entries) {
		start = len(lv.entries) - lv.maxVisible
		if start < 0 {
			start = 0
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		entry := lv.entries[i]
		timestamp := entry.Timestamp.Format("15:04:05")

		var levelStr string
		var style lipgloss.Style

		switch entry.Level {
		case DEBUG:
			levelStr = "DEBUG"
			style = debugStyle
		case INFO:
			levelStr = "INFO "
			style = infoStyle
		case ERROR:
			levelStr = "ERROR"
			style = logErrorStyle
		case SUCCESS:
			levelStr = "SUCC "
			style = logSuccessStyle
		}

		line := fmt.Sprintf("%s %s %s",
			timestampStyle.Render(timestamp),
			style.Render(levelStr),
			style.Render(entry.Message))
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < lv.maxVisible {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	containerStyle := logContainerStyle
	if lv.focused {
		containerStyle = logFocusedStyle
	}

	return containerStyle.Render(content)
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
	// Load commits for the first branch (selected by default)
	if len(m.branches) > 0 {
		m.loadCommitsForSelectedBranch()
	}
}

func (m *model) loadCommitsForSelectedBranch() {
	if len(m.branches) == 0 {
		m.selectedCommits = []Commit{}
		return
	}

	selectedRow := m.tableManager.GetCursor()
	if selectedRow >= len(m.branches) {
		m.selectedCommits = []Commit{}
		return
	}

	branchName := m.branches[selectedRow].Name
	m.logDebug("Loading commits for selected branch: %s", branchName)

	commits, err := m.gitService.GetBranchCommits(branchName, 5) // Show last 5 commits
	if err != nil {
		m.logError("Failed to load commits for branch %s: %v", branchName, err)
		m.selectedCommits = []Commit{}
		return
	}

	m.selectedCommits = commits
	m.logDebug("Loaded %d commits for branch %s", len(commits), branchName)
}

func (m *model) switchToBranch(branchName string) error {
	m.logInfo("Attempting to switch to branch: %s", branchName)

	// Get current branch to check if we're already on it
	currentBranch, err := m.gitService.GetCurrentBranch()
	if err != nil {
		m.logError("Failed to get current branch: %v", err)
		return fmt.Errorf("failed to get current branch: %v", err)
	}

	// Clean branch name (remove remote indicator)
	cleanBranchName := branchName
	if strings.HasSuffix(branchName, " (remote)") {
		cleanBranchName = strings.TrimSuffix(branchName, " (remote)")
	}

	// Check if we're already on this branch
	if cleanBranchName == currentBranch {
		m.logInfo("Already on branch %s, no switch needed", currentBranch)
		return fmt.Errorf("already on branch '%s'", currentBranch)
	}

	// Check for uncommitted changes first
	m.logDebug("Checking for uncommitted changes...")
	hasChanges, err := m.gitService.HasUncommittedChanges()
	if err != nil {
		m.logError("Failed to check for uncommitted changes: %v", err)
		return fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}

	if hasChanges {
		m.logInfo("Found uncommitted changes, showing commit modal")
		// Show modal instead of switching immediately
		m.commitModal.Show(branchName)
		return nil
	}

	m.logDebug("No uncommitted changes found, switching directly")
	// No uncommitted changes, switch directly
	err = m.gitService.SwitchToBranch(branchName)
	if err != nil {
		m.logError("Failed to switch to branch %s: %v", branchName, err)
		return err
	}

	m.logSuccess("Successfully switched to branch: %s", branchName)
	return nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle modal interactions first if modal is visible
	if m.commitModal.IsVisible() {
		m.logDebug("Modal is visible, processing modal input")
		modal, modalCmd := m.commitModal.Update(msg)
		m.commitModal = modal

		// Check if modal action was taken
		action := m.commitModal.GetAction()
		if action != ModalActionNone {
			targetBranch := m.commitModal.GetTargetBranch()
			m.logInfo("Modal action taken: %d for branch: %s", action, targetBranch)

			switch action {
			case ModalActionCommit:
				subject, description := m.commitModal.GetCommitMessage()
				m.logInfo("User chose to commit changes: '%s'", subject)
				m.logDebug("Committing changes with message: %s", subject)

				if err := m.gitService.CommitChanges(subject, description); err != nil {
					m.logError("Failed to commit changes: %v", err)
					m.message = fmt.Sprintf("Commit failed: %v", err)
				} else {
					m.logSuccess("Changes committed successfully")
					// Now switch to the target branch
					m.logDebug("Now switching to target branch: %s", targetBranch)
					if err := m.gitService.SwitchToBranch(targetBranch); err != nil {
						m.logError("Failed to switch to branch after commit: %v", err)
						m.message = fmt.Sprintf("Commit succeeded but branch switch failed: %v", err)
					} else {
						m.logSuccess("Successfully switched to branch: %s", targetBranch)
						m.message = fmt.Sprintf("Committed changes and switched to: %s", targetBranch)
						// Refresh branches to show new current branch at top
						m.logDebug("Refreshing branch list after successful switch")
						if err := m.loadBranches(); err != nil {
							m.logError("Failed to refresh branches: %v", err)
						} else {
							m.setupTable()
							m.logDebug("Branch list refreshed successfully")
						}
					}
				}

			case ModalActionStash:
				m.logInfo("User chose to stash changes")
				m.logDebug("Stashing changes for branch: %s", targetBranch)

				if err := m.gitService.StashChanges(targetBranch); err != nil {
					m.logError("Failed to stash changes: %v", err)
					m.message = fmt.Sprintf("Stash failed: %v", err)
				} else {
					m.logSuccess("Changes stashed successfully")
					// Now switch to the target branch
					m.logDebug("Now switching to target branch: %s", targetBranch)
					if err := m.gitService.SwitchToBranch(targetBranch); err != nil {
						m.logError("Failed to switch to branch after stash: %v", err)
						m.message = fmt.Sprintf("Stash succeeded but branch switch failed: %v", err)
					} else {
						m.logSuccess("Successfully switched to branch: %s", targetBranch)
						m.message = fmt.Sprintf("Stashed changes and switched to: %s", targetBranch)
						// Refresh branches to show new current branch at top
						m.logDebug("Refreshing branch list after successful switch")
						if err := m.loadBranches(); err != nil {
							m.logError("Failed to refresh branches: %v", err)
						} else {
							m.setupTable()
							m.logDebug("Branch list refreshed successfully")
						}
					}
				}

			case ModalActionCancel:
				m.logInfo("User cancelled modal - staying on current branch")
				m.message = "Branch switch cancelled"
			}

			// Hide modal after processing action
			m.commitModal.Hide()
			m.logDebug("Modal hidden after processing action")
		}

		return m, modalCmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.logInfo("User quit application")
			m.quitting = true
			return m, tea.Quit
		case "tab":
			// Toggle focus between table and logs
			m.logViewer.ToggleFocus()
			if m.logViewer.focused {
				m.logDebug("Switched focus to logs (use ↑↓ to scroll)")
			} else {
				m.logDebug("Switched focus to table")
			}
			return m, nil
		case "l":
			// Clear logs
			m.clearLogs()
			m.logInfo("Logs cleared")
			return m, nil
		case "up":
			if m.logViewer.focused {
				m.logViewer.ScrollUp()
				return m, nil
			}
		case "down":
			if m.logViewer.focused {
				m.logViewer.ScrollDown()
				return m, nil
			}
		case "enter":
			// Get selected branch and switch to it
			if len(m.branches) > 0 {
				selectedRow := m.tableManager.GetCursor()
				if selectedRow < len(m.branches) {
					branchName := m.branches[selectedRow].Name
					m.logInfo("User selected branch: %s", branchName)
					if err := m.switchToBranch(branchName); err != nil {
						m.logError("Error in switchToBranch: %v", err)
						m.message = fmt.Sprintf("Error: %v", err)
					} else {
						// Only set success message if no modal was shown
						if !m.commitModal.IsVisible() {
							m.message = fmt.Sprintf("Switched to branch: %s", branchName)
							// Refresh branches after switching - this will move the selected branch to top
							m.logDebug("Refreshing branch list after switch")
							if err := m.loadBranches(); err != nil {
								m.logError("Failed to refresh branches: %v", err)
								m.err = err
							} else {
								m.setupTable()
								m.logDebug("Branch list refreshed successfully")
							}
						}
					}
				}
			}
			return m, nil
		case "r":
			// Refresh branches
			m.logInfo("User requested branch refresh")
			m.message = "Refreshing..."
			if err := m.loadBranches(); err != nil {
				m.logError("Failed to refresh branches: %v", err)
				m.err = err
			} else {
				m.setupTable()
				m.message = "Refreshed!"
				m.logSuccess("Branch list refreshed successfully")
			}
			return m, nil
		case "c":
			// Clear message
			m.message = ""
			return m, nil
		}
	}

	// Update table only if logs are not focused
	if !m.logViewer.focused {
		oldCursor := m.tableManager.GetCursor()
		table, tableCmd := m.tableManager.UpdateTable(msg)
		m.tableManager.table = table
		cmd = tableCmd

		// Check if cursor position changed to load commits for new selection
		newCursor := m.tableManager.GetCursor()
		if oldCursor != newCursor {
			m.loadCommitsForSelectedBranch()
		}
	}

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

	// Commit preview section
	commitPreview := m.renderCommitPreview()

	// Log section title with focus indicator
	var logTitle string
	if m.logViewer.focused {
		logTitle = logTitleStyle.Render("Debug Logs: [FOCUSED - ↑↓ to scroll]")
	} else {
		logTitle = logTitleStyle.Render("Debug Logs:")
	}

	// Help text with new shortcuts
	help := helpStyle.Render("↑/↓: navigate/scroll • enter: switch • tab: focus logs • l: clear logs • r: refresh • q: quit")

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
		commitPreview,
		"",
		logTitle,
		m.logViewer.View(),
		"",
		messageView,
		help,
	)

	// Show modal overlay if modal is visible
	if m.commitModal.IsVisible() {
		return m.commitModal.ViewOverlay(content)
	}

	return content
}

func (m model) renderCommitPreview() string {
	if len(m.branches) == 0 {
		return commitContainerStyle.Render("No branches available")
	}

	selectedRow := m.tableManager.GetCursor()
	if selectedRow >= len(m.branches) {
		return commitContainerStyle.Render("No branch selected")
	}

	branchName := m.branches[selectedRow].Name
	commitTitle := commitTitleStyle.Render(fmt.Sprintf("Recent Commits - %s:", branchName))

	if len(m.selectedCommits) == 0 {
		emptyMsg := "No commits found or loading..."
		return commitContainerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, commitTitle, "", emptyMsg))
	}

	var commitLines []string
	commitLines = append(commitLines, commitTitle)
	commitLines = append(commitLines, "")

	for _, commit := range m.selectedCommits {
		commitLine := fmt.Sprintf("%s %s %s - %s",
			commitHashStyle.Render(commit.Hash),
			commitTimeStyle.Render(commit.RelativeTime),
			commitAuthorStyle.Render(commit.Author),
			truncateString(commit.Subject, 50))
		commitLines = append(commitLines, commitLine)
	}

	// Pad with empty lines to maintain consistent height
	for len(commitLines) < 6 {
		commitLines = append(commitLines, "")
	}

	content := strings.Join(commitLines, "\n")
	return commitContainerStyle.Render(content)
}

func getRemoteText(includeRemote bool) string {
	if includeRemote {
		return "local + remote"
	}
	return "local only"
}

// Test change for modal
