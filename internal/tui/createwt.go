package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gdelco/lazypilot/internal/codename"
	"github.com/gdelco/lazypilot/internal/gitctl"
)

// createStep identifies the current wizard step.
type createStep int

const (
	stepBranch    createStep = iota // text input — branch name (pre-filled with codename)
	stepContainer                   // list with type-ahead — where the worktree dir lives
	stepPath                        // text input — final worktree path
	stepBase                        // list with type-ahead — base ref
	stepConfirm                     // summary + execute
)

// createWizard is a multi-step modal for creating a git worktree.
type createWizard struct {
	repo string // main repo path (resolved from selection if a worktree was selected)

	codename   string
	branchInput textinput.Model

	containerOptions []string
	containerInput   textinput.Model
	containerCursor  int

	pathInput textinput.Model

	baseOptions []string
	baseInput   textinput.Model
	baseCursor  int

	step      createStep
	doneState string // "" while running, "ok"/"err: …" after execute
	doneMsg   string
}

func newCreateWizard(repo string, branchPrefix string) *createWizard {
	// Resolve to main repo if selection was a worktree.
	repo = gitctl.MainWorktree(repo)

	name := codename.New()
	current := gitctl.CurrentBranch(repo)

	w := &createWizard{repo: repo, codename: name}

	// Branch input
	w.branchInput = textinput.New()
	w.branchInput.Prompt = ""
	w.branchInput.SetValue(branchPrefix + name)
	w.branchInput.Focus()
	w.branchInput.Width = 60

	// Container options
	parent := filepath.Dir(repo)
	leaf := filepath.Base(repo)
	w.containerOptions = uniq([]string{
		filepath.Join(parent, "worktrees", leaf),
		filepath.Join(parent, "worktrees"),
		filepath.Join(parent, leaf+"-worktrees"),
		parent,
	})
	w.containerInput = textinput.New()
	w.containerInput.Prompt = ""
	w.containerInput.Width = 60

	// Path input (filled later from container + codename)
	w.pathInput = textinput.New()
	w.pathInput.Prompt = ""
	w.pathInput.Width = 70

	// Base options — full list of branches; baseInput starts empty so the
	// step behaves like a search: showing all branches, narrowing as the user types.
	w.baseOptions = gitctl.ListBranches(repo)
	w.baseInput = textinput.New()
	w.baseInput.Prompt = ""
	w.baseInput.Placeholder = "type to filter (blank = " + current + ")"
	w.baseInput.Width = 60
	_ = current

	return w
}

// filteredBase returns the branches matching the current baseInput value
// (case-insensitive substring). Empty input returns all options.
func (w *createWizard) filteredBase() []string {
	query := strings.ToLower(strings.TrimSpace(w.baseInput.Value()))
	if query == "" {
		return w.baseOptions
	}
	out := []string{}
	for _, opt := range w.baseOptions {
		if strings.Contains(strings.ToLower(opt), query) {
			out = append(out, opt)
		}
	}
	return out
}

func (w *createWizard) Update(msg tea.Msg) (cmd tea.Cmd, done bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	// Escape cancels at any step.
	if km.String() == "esc" {
		return nil, true
	}

	switch w.step {
	case stepBranch:
		return w.updateBranch(km)
	case stepContainer:
		return w.updateContainer(km)
	case stepPath:
		return w.updatePath(km)
	case stepBase:
		return w.updateBase(km)
	case stepConfirm:
		return w.updateConfirm(km)
	}
	return nil, false
}

func (w *createWizard) updateBranch(km tea.KeyMsg) (tea.Cmd, bool) {
	if km.String() == "enter" {
		if strings.TrimSpace(w.branchInput.Value()) == "" {
			return nil, false
		}
		w.step = stepContainer
		w.containerInput.SetValue(w.containerOptions[0])
		w.containerInput.Focus()
		w.branchInput.Blur()
		return nil, false
	}
	var cmd tea.Cmd
	w.branchInput, cmd = w.branchInput.Update(km)
	return cmd, false
}

func (w *createWizard) updateContainer(km tea.KeyMsg) (tea.Cmd, bool) {
	switch km.String() {
	case "enter":
		picked := w.containerInput.Value()
		if picked == "" && len(w.containerOptions) > 0 {
			picked = w.containerOptions[w.containerCursor]
		}
		picked = expandHomeStr(picked)
		w.step = stepPath
		// Pre-fill final path = container/codename
		w.pathInput.SetValue(filepath.Join(picked, w.codename))
		w.pathInput.Focus()
		w.containerInput.Blur()
		return nil, false
	case "up", "ctrl+p":
		if w.containerCursor > 0 {
			w.containerCursor--
			w.containerInput.SetValue(w.containerOptions[w.containerCursor])
		}
		return nil, false
	case "down", "ctrl+n":
		if w.containerCursor < len(w.containerOptions)-1 {
			w.containerCursor++
			w.containerInput.SetValue(w.containerOptions[w.containerCursor])
		}
		return nil, false
	}
	var cmd tea.Cmd
	w.containerInput, cmd = w.containerInput.Update(km)
	return cmd, false
}

func (w *createWizard) updatePath(km tea.KeyMsg) (tea.Cmd, bool) {
	if km.String() == "enter" {
		if strings.TrimSpace(w.pathInput.Value()) == "" {
			return nil, false
		}
		w.step = stepBase
		w.baseInput.Focus()
		w.baseInput.SetValue("") // start empty so the search shows everything
		w.baseCursor = 0
		w.pathInput.Blur()
		return nil, false
	}
	var cmd tea.Cmd
	w.pathInput, cmd = w.pathInput.Update(km)
	return cmd, false
}

func (w *createWizard) updateBase(km tea.KeyMsg) (tea.Cmd, bool) {
	switch km.String() {
	case "enter":
		// If the highlighted item exists and matches the cursor, prefer it.
		// Otherwise fall back to the raw typed text (or current branch when blank).
		filtered := w.filteredBase()
		if len(filtered) > 0 && w.baseCursor >= 0 && w.baseCursor < len(filtered) {
			w.baseInput.SetValue(filtered[w.baseCursor])
		}
		if strings.TrimSpace(w.baseInput.Value()) == "" {
			w.baseInput.SetValue(gitctl.CurrentBranch(w.repo))
		}
		w.step = stepConfirm
		return nil, false
	case "up", "ctrl+p":
		if w.baseCursor > 0 {
			w.baseCursor--
		}
		return nil, false
	case "down", "ctrl+n":
		filtered := w.filteredBase()
		if w.baseCursor < len(filtered)-1 {
			w.baseCursor++
		}
		return nil, false
	}
	// Any other key edits the input; reset cursor to 0 so the top match is highlighted.
	var cmd tea.Cmd
	w.baseInput, cmd = w.baseInput.Update(km)
	w.baseCursor = 0
	return cmd, false
}

func (w *createWizard) updateConfirm(km tea.KeyMsg) (tea.Cmd, bool) {
	switch km.String() {
	case "enter", "y", "Y":
		return w.execute(), true
	}
	return nil, false
}

// execute runs `git worktree add`. Returns a tea.Cmd that emits a rescanMsg
// when done so the main views refresh.
func (w *createWizard) execute() tea.Cmd {
	path := expandHomeStr(w.pathInput.Value())
	branch := w.branchInput.Value()
	base := w.baseInput.Value()
	repo := w.repo

	return func() tea.Msg {
		// Make sure parent directory of `path` exists.
		if parent := filepath.Dir(path); parent != "" {
			_ = os.MkdirAll(parent, 0o755)
		}
		if err := gitctl.AddWorktree(repo, path, branch, base); err != nil {
			return rescanMsg{status: "create failed: " + err.Error()}
		}
		return rescanMsg{status: "worktree created: " + path}
	}
}

// View renders the wizard centered on screen.
func (w *createWizard) View(s styles, screenW, screenH int) string {
	width := 78
	if width > screenW-6 {
		width = screenW - 6
	}

	var body strings.Builder
	stepLabel := func(n int, label string) string {
		mark := s.Dim.Render("○")
		if n < int(w.step)+1 {
			mark = s.OK.Render("●")
		}
		if n == int(w.step)+1 {
			mark = s.Heading.Render("●")
			label = lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(label)
		} else {
			label = s.Dim.Render(label)
		}
		return mark + " " + label
	}
	body.WriteString(strings.Join([]string{
		stepLabel(1, "branch"),
		stepLabel(2, "container"),
		stepLabel(3, "path"),
		stepLabel(4, "base"),
		stepLabel(5, "confirm"),
	}, "   "))
	body.WriteString("\n\n")

	switch w.step {
	case stepBranch:
		body.WriteString(s.Heading.Render("Branch name") + "\n")
		body.WriteString(w.branchInput.View() + "\n")
	case stepContainer:
		body.WriteString(s.Heading.Render("Container directory") + "\n")
		body.WriteString(w.containerInput.View() + "\n\n")
		body.WriteString(s.Dim.Render("suggestions:") + "\n")
		for i, opt := range w.containerOptions {
			if i == w.containerCursor {
				body.WriteString("  " + s.ListSelected.Render("▶ "+opt) + "\n")
			} else {
				body.WriteString("    " + opt + "\n")
			}
		}
	case stepPath:
		body.WriteString(s.Heading.Render("Worktree path") + "\n")
		body.WriteString(w.pathInput.View() + "\n")
		body.WriteString("\n" + s.Dim.Render("the directory git will create for this worktree.") + "\n")
	case stepBase:
		body.WriteString(s.Heading.Render("Base ref (fork from)") + "\n")
		body.WriteString(w.baseInput.View() + "\n\n")
		filtered := w.filteredBase()
		body.WriteString(s.Dim.Render(fmt.Sprintf("branches (%d/%d) — type to filter, ↑/↓ to pick, enter to choose:", len(filtered), len(w.baseOptions))) + "\n")
		if len(filtered) == 0 {
			body.WriteString("  " + s.Dim.Render("(no branches match — press enter to use typed value as a custom ref)") + "\n")
		} else {
			windowSize := 10
			start := w.baseCursor - windowSize/2
			if start < 0 {
				start = 0
			}
			end := start + windowSize
			if end > len(filtered) {
				end = len(filtered)
				start = end - windowSize
				if start < 0 {
					start = 0
				}
			}
			for i := start; i < end; i++ {
				opt := filtered[i]
				if i == w.baseCursor {
					body.WriteString("  " + s.ListSelected.Render("▶ "+opt) + "\n")
				} else {
					body.WriteString("    " + opt + "\n")
				}
			}
		}
	case stepConfirm:
		body.WriteString(s.Heading.Render("Confirm") + "\n\n")
		body.WriteString(fmt.Sprintf("  repo:    %s\n", w.repo))
		body.WriteString(fmt.Sprintf("  branch:  %s\n", w.branchInput.Value()))
		body.WriteString(fmt.Sprintf("  path:    %s\n", w.pathInput.Value()))
		body.WriteString(fmt.Sprintf("  base:    %s\n\n", w.baseInput.Value()))
		body.WriteString(s.OK.Render("[enter] create   ") + s.Dim.Render("[esc] cancel") + "\n")
	}

	if w.step != stepConfirm {
		body.WriteString("\n" + s.Dim.Render("[enter] next   [esc] cancel"))
	}

	box := s.Panel("NEW WORKTREE", body.String(), width, lipgloss.Height(body.String()), true)
	return lipgloss.Place(screenW, screenH, lipgloss.Center, lipgloss.Center, box)
}

// --- helpers ---

func uniq(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func expandHomeStr(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
