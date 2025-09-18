package main

import (
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Table styles
	tableStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
)

type TableManager struct {
	table table.Model
}

func NewTableManager() *TableManager {
	return &TableManager{}
}

func (tm *TableManager) SetupTable(branches []Branch) {
	columns := []table.Column{
		{Title: "Branch", Width: 35},
		{Title: "Last Used", Width: 15},
		{Title: "Last Commit", Width: 12},
		{Title: "Commit Message", Width: 60},
	}

	rows := make([]table.Row, 0, len(branches))
	for _, branch := range branches {
		// Update relative times
		branch.RelativeTime = formatLastUsedTime(branch.LastUsed)

		commitMsg := truncateString(branch.CommitTitle, 57)
		commitDate := branch.CommitDate.Format("2006-01-02")

		row := table.Row{
			branch.Name,
			branch.RelativeTime,
			commitDate,
			commitMsg,
		}
		rows = append(rows, row)
	}

	// Ensure we have at least one row to avoid empty table issues
	if len(rows) == 0 {
		rows = append(rows, table.Row{"No branches found", "", "", ""})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(len(rows), 20)), // Limit max height
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false).
		Foreground(lipgloss.Color("229"))

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	tm.table = t
}

func (tm *TableManager) GetTable() table.Model {
	return tm.table
}

func (tm *TableManager) UpdateTable(msg interface{}) (table.Model, tea.Cmd) {
	return tm.table.Update(msg)
}

func (tm *TableManager) GetCursor() int {
	return tm.table.Cursor()
}

func (tm *TableManager) View() string {
	return tableStyle.Render(tm.table.View())
}
