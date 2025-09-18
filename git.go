package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Branch struct {
	Name         string
	CommitDate   time.Time
	CommitTitle  string
	LastUsed     time.Time // When this branch was last checked out
	IsRemote     bool
	RelativeTime string
}

type GitService struct{}

func NewGitService() *GitService {
	return &GitService{}
}

func (g *GitService) IsInRepository() error {
	return exec.Command("git", "rev-parse", "--git-dir").Run()
}

func (g *GitService) GetRecentBranches(count int, includeRemote bool, authors []string) ([]Branch, error) {
	if err := g.IsInRepository(); err != nil {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Get current user for "mine" filtering
	var currentUser string
	if len(authors) > 0 && authors[0] == "mine" {
		var err error
		currentUser, err = g.GetCurrentUser()
		if err != nil {
			return nil, fmt.Errorf("failed to get current user: %v", err)
		}
	}

	// Get reflog information to find when branches were last used
	reflogCmd := exec.Command("git", "reflog", "--all", "--grep-reflog=checkout:", "--format=%gd|%gs|%gt")
	reflogOutput, err := reflogCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git reflog: %v", err)
	}

	// Parse reflog to get last checkout times
	branchLastUsed := make(map[string]time.Time)
	reflogLines := strings.Split(strings.TrimSpace(string(reflogOutput)), "\n")

	for _, line := range reflogLines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}

		subject := parts[1]
		timestampStr := parts[2]

		// Parse checkout operations: "checkout: moving from branch1 to branch2"
		if strings.Contains(subject, "checkout: moving from") {
			// Extract the target branch (the one we moved TO)
			if strings.Contains(subject, " to ") {
				targetBranch := strings.TrimSpace(strings.Split(subject, " to ")[1])

				// Parse timestamp
				timestamp, err := parseTimestamp(timestampStr)
				if err != nil {
					continue
				}

				// Only record if this is the most recent checkout for this branch
				if existing, exists := branchLastUsed[targetBranch]; !exists || timestamp.After(existing) {
					branchLastUsed[targetBranch] = timestamp
				}
			}
		}
	}

	// Get current branch to ensure it's at the top
	currentBranch, _ := g.GetCurrentBranch()
	if currentBranch != "" {
		branchLastUsed[currentBranch] = time.Now()
	}

	// Get branch information
	localBranches, err := g.getBranchInfo("refs/heads/")
	if err != nil {
		return nil, err
	}

	var allBranches []Branch
	allBranches = append(allBranches, localBranches...)

	// Add remote branches if requested
	if includeRemote {
		remoteBranches, _ := g.getBranchInfo("refs/remotes/")
		allBranches = append(allBranches, remoteBranches...)
	}

	// Filter by authors if specified (main/master will always pass this filter)
	var filteredBranches []Branch
	if len(authors) > 0 && authors[0] != "all" {
		fmt.Printf("DEBUG: Filtering %d branches by authors: %v (currentUser: %s)\n", len(allBranches), authors, currentUser)
		for _, branch := range allBranches {
			fmt.Printf("DEBUG: Checking branch '%s'...\n", branch.Name)
			shouldInclude, err := g.branchHasAuthorCommits(branch, authors, currentUser)
			if err != nil {
				fmt.Printf("DEBUG: Error checking branch '%s': %v - including anyway\n", branch.Name, err)
				// If we can't determine authorship, include the branch
				filteredBranches = append(filteredBranches, branch)
				continue
			}
			if shouldInclude {
				fmt.Printf("DEBUG: Including branch '%s'\n", branch.Name)
				filteredBranches = append(filteredBranches, branch)
			} else {
				fmt.Printf("DEBUG: Excluding branch '%s' (no matching author commits)\n", branch.Name)
			}
		}
		fmt.Printf("DEBUG: After filtering: %d branches remain\n", len(filteredBranches))
	} else {
		fmt.Printf("DEBUG: No author filtering - showing all %d branches\n", len(allBranches))
		filteredBranches = allBranches
	}

	// Set last used times for all branches
	var branches []Branch
	for _, branch := range filteredBranches {
		branchKey := branch.Name
		if branch.IsRemote {
			// Remove " (remote)" suffix for lookup
			branchKey = strings.TrimSuffix(branch.Name, " (remote)")
		}

		if lastUsed, exists := branchLastUsed[branchKey]; exists {
			branch.LastUsed = lastUsed
		} else {
			// If no reflog entry, use commit date as fallback
			branch.LastUsed = branch.CommitDate
		}
		branches = append(branches, branch)
	}

	// Sort by last used time (most recent first)
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].LastUsed.After(branches[j].LastUsed)
	})

	// Limit to requested count
	if len(branches) > count {
		branches = branches[:count]
	}

	return branches, nil
}

func (g *GitService) getBranchInfo(refPath string) ([]Branch, error) {
	cmd := exec.Command("git", "for-each-ref",
		"--sort=-committerdate",
		"--format=%(refname:short)|%(committerdate:iso8601)|%(contents:subject)",
		refPath)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git branches for %s: %v", refPath, err)
	}

	if strings.TrimSpace(string(output)) == "" {
		return []Branch{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []Branch

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}

		branchName := strings.TrimSpace(parts[0])
		dateStr := strings.TrimSpace(parts[1])
		commitTitle := strings.TrimSpace(parts[2])

		if branchName == "" {
			continue
		}

		// Parse the commit date
		commitDate, err := parseGitDate(dateStr)
		if err != nil {
			commitDate = time.Now() // Fallback
		}

		isRemote := strings.HasPrefix(branchName, "origin/")
		displayName := branchName
		if isRemote {
			displayName = strings.TrimPrefix(branchName, "origin/") + " (remote)"
		}

		branch := Branch{
			Name:        displayName,
			CommitDate:  commitDate,
			CommitTitle: commitTitle,
			IsRemote:    isRemote,
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

func (g *GitService) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (g *GitService) HasUncommittedChanges() (bool, error) {
	// Check for staged changes
	stagedCmd := exec.Command("git", "diff", "--cached", "--quiet")
	stagedErr := stagedCmd.Run()

	// Check for unstaged changes
	unstagedCmd := exec.Command("git", "diff", "--quiet")
	unstagedErr := unstagedCmd.Run()

	// If either command returns non-zero, there are changes
	hasChanges := stagedErr != nil || unstagedErr != nil

	return hasChanges, nil
}

// GitFileStatus represents the status of a file in git
type GitFileStatus struct {
	Path         string
	Status       string // M, A, D, R, C, U, etc.
	StagedStatus string // Status in index
	WorkStatus   string // Status in working tree
	LinesAdded   int    // Number of lines added
	LinesDeleted int    // Number of lines deleted
}

// GetGitStatus returns detailed git status information
func (g *GitService) GetGitStatus() ([]GitFileStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain=v1")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %v", err)
	}

	var files []GitFileStatus
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		stagedStatus := string(line[0])
		workStatus := string(line[1])
		path := strings.TrimSpace(line[2:])

		// Determine overall status
		status := "M" // Modified by default
		if stagedStatus == "A" || workStatus == "A" {
			status = "A" // Added
		} else if stagedStatus == "D" || workStatus == "D" {
			status = "D" // Deleted
		} else if stagedStatus == "R" || workStatus == "R" {
			status = "R" // Renamed
		} else if stagedStatus == "C" || workStatus == "C" {
			status = "C" // Copied
		} else if stagedStatus == "U" || workStatus == "U" {
			status = "U" // Unmerged
		}

		// Get line statistics for this file
		linesAdded, linesDeleted := g.getFileLineStats(path, stagedStatus, workStatus)

		files = append(files, GitFileStatus{
			Path:         path,
			Status:       status,
			StagedStatus: stagedStatus,
			WorkStatus:   workStatus,
			LinesAdded:   linesAdded,
			LinesDeleted: linesDeleted,
		})
	}

	return files, nil
}

// getFileLineStats returns the number of lines added and deleted for a specific file
func (g *GitService) getFileLineStats(filePath, stagedStatus, workStatus string) (int, int) {
	var totalAdded, totalDeleted int

	// Get staged changes stats
	if stagedStatus != " " && stagedStatus != "?" {
		added, deleted := g.getNumstatForFile(filePath, true)
		totalAdded += added
		totalDeleted += deleted
	}

	// Get unstaged changes stats
	if workStatus != " " && workStatus != "?" {
		added, deleted := g.getNumstatForFile(filePath, false)
		totalAdded += added
		totalDeleted += deleted
	}

	return totalAdded, totalDeleted
}

// getNumstatForFile gets numstat for a specific file (staged or unstaged)
func (g *GitService) getNumstatForFile(filePath string, staged bool) (int, int) {
	var cmd *exec.Cmd
	if staged {
		cmd = exec.Command("git", "diff", "--numstat", "--cached", filePath)
	} else {
		cmd = exec.Command("git", "diff", "--numstat", filePath)
	}

	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	return g.parseNumstatOutput(string(output))
}

// parseNumstatOutput parses the output of git diff --numstat
func (g *GitService) parseNumstatOutput(output string) (int, int) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var totalAdded, totalDeleted int

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Handle binary files (marked with "-")
		if parts[0] == "-" || parts[1] == "-" {
			continue
		}

		// Parse added lines
		if added := parseInt(parts[0]); added >= 0 {
			totalAdded += added
		}

		// Parse deleted lines
		if deleted := parseInt(parts[1]); deleted >= 0 {
			totalDeleted += deleted
		}
	}

	return totalAdded, totalDeleted
}

// parseInt safely parses an integer, returning -1 on error
func parseInt(s string) int {
	var result int
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		result = result*10 + int(r-'0')
	}
	return result
}

// GetFileDiff returns the diff for a specific file
func (g *GitService) GetFileDiff(filePath string) (string, error) {
	// Get both staged and unstaged changes
	stagedCmd := exec.Command("git", "diff", "--cached", filePath)
	stagedOutput, _ := stagedCmd.Output()

	unstagedCmd := exec.Command("git", "diff", filePath)
	unstagedOutput, _ := unstagedCmd.Output()

	diff := ""
	if len(stagedOutput) > 0 {
		diff += "=== Staged Changes ===\n" + string(stagedOutput) + "\n"
	}
	if len(unstagedOutput) > 0 {
		diff += "=== Unstaged Changes ===\n" + string(unstagedOutput) + "\n"
	}

	if diff == "" {
		return "No changes to display", nil
	}

	return diff, nil
}

func (g *GitService) CommitChanges(subject, description string) error {
	// Stage all changes first
	stageCmd := exec.Command("git", "add", "-A")
	if err := stageCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage changes: %v", err)
	}

	// Prepare commit message
	var message string
	if strings.TrimSpace(description) != "" {
		message = subject + "\n\n" + description
	} else {
		message = subject
	}

	// Commit changes
	commitCmd := exec.Command("git", "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("failed to commit changes: %v", err)
	}

	return nil
}

func (g *GitService) StashChanges(branchName string) error {
	// Create a descriptive stash message
	stashMessage := fmt.Sprintf("WIP: changes before switching to %s", branchName)

	stashCmd := exec.Command("git", "stash", "push", "-m", stashMessage)
	if err := stashCmd.Run(); err != nil {
		return fmt.Errorf("failed to stash changes: %v", err)
	}

	return nil
}

func (g *GitService) SwitchToBranch(branchName string) error {
	// Remove remote indicator for display
	actualBranchName := branchName
	isRemote := strings.HasSuffix(branchName, " (remote)")

	if isRemote {
		// For remote branches, remove the (remote) suffix
		actualBranchName = strings.TrimSuffix(branchName, " (remote)")

		// Check if local branch exists
		checkCmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+actualBranchName)
		if checkCmd.Run() != nil {
			// Local branch doesn't exist, create and track it
			createCmd := exec.Command("git", "checkout", "-b", actualBranchName, "origin/"+actualBranchName)
			output, err := createCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to create and checkout branch %s: %v\nOutput: %s", actualBranchName, err, string(output))
			}
			return nil
		}
	}

	// Switch to existing local branch
	cmd := exec.Command("git", "checkout", actualBranchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout branch %s: %v\nOutput: %s", actualBranchName, err, string(output))
	}

	return nil
}

func (g *GitService) GetCurrentUser() (string, error) {
	cmd := exec.Command("git", "config", "user.email")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to user.name if email not available
		cmd = exec.Command("git", "config", "user.name")
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(output)), nil
}

func (g *GitService) branchHasAuthorCommits(branch Branch, authors []string, currentUser string) (bool, error) {
	branchName := branch.Name
	if branch.IsRemote {
		branchName = strings.TrimSuffix(branchName, " (remote)")
	}

	fmt.Printf("DEBUG: branchHasAuthorCommits for '%s' (original: '%s')\n", branchName, branch.Name)

	// Always include main/master branches regardless of author filtering
	if branchName == "main" || branchName == "master" {
		fmt.Printf("DEBUG: Branch '%s' is main/master - including\n", branchName)
		return true, nil
	}

	// For remote branches, adjust the branch name for git commands
	gitBranchName := branchName
	if branch.IsRemote {
		gitBranchName = "origin/" + branchName
	}

	fmt.Printf("DEBUG: Using git branch name '%s' for commands\n", gitBranchName)

	// Find the merge base with main/master to see commits unique to this branch
	mergeBase, err := g.findMergeBase(gitBranchName)
	if err != nil {
		fmt.Printf("DEBUG: Could not find merge base for '%s': %v - including branch\n", gitBranchName, err)
		// If we can't find merge base, include the branch
		return true, nil
	}

	fmt.Printf("DEBUG: Found merge base for '%s': %s\n", gitBranchName, mergeBase)

	// Get commits that are in this branch but not in the base branch
	cmd := exec.Command("git", "log", "--format=%ae|%an", mergeBase+".."+gitBranchName)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("DEBUG: Could not get commits for '%s': %v - including branch\n", gitBranchName, err)
		// If we can't get commits, include the branch
		return true, nil
	}

	if strings.TrimSpace(string(output)) == "" {
		fmt.Printf("DEBUG: No unique commits in branch '%s' - but including anyway (branch exists)\n", branchName)
		// No unique commits in this branch, but we'll include it anyway since it's a valid branch
		// This handles cases where branches have been merged or are at the same point as main
		return true, nil
	}

	commitLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	fmt.Printf("DEBUG: Found %d unique commits in branch '%s'\n", len(commitLines), branchName)

	// Check if any commits match our authors
	for _, line := range commitLines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}

		email := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])

		fmt.Printf("DEBUG: Checking commit author: email='%s', name='%s'\n", email, name)

		// Check against our author filters
		for _, author := range authors {
			if author == "mine" {
				if email == currentUser || name == currentUser {
					fmt.Printf("DEBUG: Found matching 'mine' commit in '%s' - including\n", branchName)
					return true, nil
				}
			} else {
				if strings.Contains(strings.ToLower(email), strings.ToLower(author)) ||
					strings.Contains(strings.ToLower(name), strings.ToLower(author)) {
					fmt.Printf("DEBUG: Found matching author '%s' commit in '%s' - including\n", author, branchName)
					return true, nil
				}
			}
		}
	}

	fmt.Printf("DEBUG: No matching author commits found in '%s' - excluding\n", branchName)
	return false, nil
}

func (g *GitService) findMergeBase(branchName string) (string, error) {
	// Try common base branches
	baseBranches := []string{"main", "master", "develop", "dev"}

	for _, base := range baseBranches {
		cmd := exec.Command("git", "merge-base", base, branchName)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) != "" {
			return strings.TrimSpace(string(output)), nil
		}
	}

	// Fallback: use the first commit in the repository
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func parseGitDate(dateStr string) (time.Time, error) {
	// Try different date formats
	formats := []string{
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05+00:00",
		"2006-01-02 15:04:05 -0700",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

func parseTimestamp(timestampStr string) (time.Time, error) {
	// Git reflog timestamps are Unix timestamps
	return time.Parse("1136239445", timestampStr)
}
