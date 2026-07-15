package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/N1xev/spin/internal/template"
)

// hookItem adapts a template.HookView to the bubbles list.Item interface.
type hookItem struct {
	template.HookView
}

func (i hookItem) FilterValue() string { return i.Title() }

func (i hookItem) Title() string {
	if i.IsFile {
		return fmt.Sprintf("%s  %s", i.Phase, filepath.Base(i.File))
	}
	return fmt.Sprintf("%s  %s", i.Phase, i.Run)
}

func (i hookItem) Description() string {
	if i.IsFile {
		return i.File
	}
	return "inline command"
}

// runLineMsg delivers one chunk of streamed hook output.
type runLineMsg struct{ text string }

// runDoneMsg signals the hook run finished; err carries any failure.
type runDoneMsg struct{ err error }

// chanWriter forwards writes to a channel so hook output can be
// streamed into the bubbletea event loop.
type chanWriter struct{ ch chan string }

func (w *chanWriter) Write(p []byte) (int, error) {
	w.ch <- string(p)
	return len(p), nil
}

const hookHintShort = "enter preview hook • R run all + scaffold • ←/→ focus • q/esc quit"

// hooksModel renders the interactive hook review screen. The left pane
// lists every hook the template will run; the right pane streams the
// live output when the hooks execute. Pressing Enter opens a centered
// Run/Skip modal (replacing the CLI trust prompt); confirming Run or
// Skip executes the full scaffold (pre → render → post) and streams its
// output into the right pane.
type hooksModel struct {
	styles   *tuiStyles
	tpl      *template.Template
	ctx      context.Context
	dest     string
	name     string
	resolved map[string]any
	list     list.Model
	viewport viewport.Model
	hooks    []template.HookView
	width    int
	height   int
	focus    string // "list" or "view"
	selected int

	modalOpen   bool
	modalChoice int // 0 Run, 1 Skip

	running    bool
	runningAll bool // running the full scaffold (vs a single hook)
	output     string
	didRun     bool // scaffold was executed by this model
	stream     chan string
	doneCh     chan error

	verbose  bool
	autoStart int // 0 none, 1 run, 2 skip
}

func newHooksModel(tpl *template.Template, styles *tuiStyles, width, height int, resolved map[string]any, ctx context.Context, dest, name string, noHooks, yes, verbose bool) hooksModel {
	items := template.CollectHooks(tpl)
	listItems := make([]list.Item, 0, len(items))
	for _, h := range items {
		listItems = append(listItems, hookItem{h})
	}

	delegate := list.NewDefaultDelegate()
	listH := max(height-7, 1)
	listW := 42
	l := list.New(listItems, delegate, listW, listH)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.SetSize(listW, listH)
	if len(listItems) > 0 {
		l.Select(0)
	}

	viewW := max(width-listW-3, 20)
	vp := viewport.New(viewport.WithWidth(viewW), viewport.WithHeight(listH))

	m := hooksModel{
		styles:   styles,
		tpl:      tpl,
		ctx:      ctx,
		dest:     dest,
		name:     name,
		resolved: resolved,
		list:     l,
		viewport: vp,
		hooks:    items,
		width:    width,
		height:   height,
		focus:    "list",
		selected: -1,
		verbose:  verbose,
	}
	m.viewport.SetContent(m.selectedHookContent(m.list.Index()))
	m.viewport.GotoTop()
	if noHooks {
		m.autoStart = 2
	} else if yes {
		m.autoStart = 1
	}
	return m
}

// selectedHookContent builds the right-pane content shown while the
// user is reviewing hooks (before pressing Enter to run): the inline
// [[pre]]/[[post]] command or the contents of the _pre/_post script
// file for the hook at idx. It is replaced by streamed command +
// output once a hook actually runs.
func (m hooksModel) selectedHookContent(idx int) string {
	var b strings.Builder
	if idx < 0 || idx >= len(m.hooks) {
		fmt.Fprintln(&b, "Select a hook to preview its command or script.")
		fmt.Fprintf(&b, "\n%s\n", hookHintShort)
		return b.String()
	}
	h := m.hooks[idx]
	header := fmt.Sprintf("Hook %d — %s", idx+1, h.Phase)
	if h.IsFile {
		data, err := os.ReadFile(h.File)
		body := ""
		if err != nil {
			body = fmt.Sprintf("(could not read %s: %v)", filepath.Base(h.File), err)
		} else {
			body = string(data)
		}
		fmt.Fprintf(&b, "%s  file  %s\n\n", header, filepath.Join("_"+h.Phase, filepath.Base(h.File)))
		b.WriteString(body)
	} else {
		fmt.Fprintf(&b, "%s  inline\n\n", header)
		fmt.Fprintf(&b, "  %s\n", h.Run)
	}
	fmt.Fprintf(&b, "\n%s\n", hookHintShort)
	return b.String()
}

func (m hooksModel) update(msg tea.Msg) (hooksModel, tea.Cmd) {
	if m.autoStart != 0 && !m.running && !m.modalOpen {
		skip := m.autoStart == 2
		m.autoStart = 0
		return m.startRun(skip)
	}

	switch msg := msg.(type) {
	case runLineMsg:
		m.output += msg.text
		m.viewport.SetContent(m.output)
		m.viewport.GotoBottom()
		return m, m.listen()
	case runDoneMsg:
		m.running = false
		if msg.err != nil {
			m.output += "\n" + lipgloss.NewStyle().Foreground(tuiBrightRed).Render("error: "+msg.err.Error())
		} else {
			m.output += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("done.")
			// Mirror the success summary into the pane so the user
			// sees it before quitting. runNew reprints the same two
			// lines on the restored terminal once they press q.
			if m.runningAll {
				m.output += "\n"
				m.output += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(
					fmt.Sprintf("INFO created %s at %s", m.name, m.dest))
				m.output += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
					fmt.Sprintf("cd %s", m.dest))
			}
		}
		m.viewport.SetContent(m.output)
		m.viewport.GotoBottom()
		return m, nil
	case tea.KeyPressMsg:
		if m.modalOpen {
			switch msg.String() {
			case "left", "h", "tab":
				m.modalChoice = 0
				return m, nil
			case "right", "l":
				m.modalChoice = 1
				return m, nil
			case "enter":
				return m.submitModal()
			case "y":
				m.modalChoice = 0
				return m.submitModal()
			case "n":
				m.modalChoice = 1
				return m.submitModal()
			case "esc", "q":
				m.modalOpen = false
				return m, nil
			}
			return m, nil
		}
		if m.running {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Interrupt
			case "esc", "q":
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "enter":
			// Run the selected hook in place: render its command
			// and stream the output into the right pane.
			return m.startSingleRun(m.list.Index())
		case "R", "a":
			// Run every hook (full scaffold) via the modal.
			m.modalOpen = true
			return m, nil
		case "ctrl+c":
			return m, tea.Interrupt
		case "esc", "q":
			return m, tea.Quit
		case "left", "right", "tab":
			if m.focus == "list" {
				m.focus = "view"
			} else {
				m.focus = "list"
				m.viewport.SetContent(m.selectedHookContent(m.list.Index()))
				m.viewport.GotoTop()
			}
			return m, nil
		}
		if m.focus == "view" {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if !m.running && !m.modalOpen && m.focus == "list" {
			m.viewport.SetContent(m.selectedHookContent(m.list.Index()))
			m.viewport.GotoTop()
		}
		return m, cmd
	}
	return m, nil
}

// submitModal resolves the current modal choice into a hook run.
func (m hooksModel) submitModal() (hooksModel, tea.Cmd) {
	m.modalOpen = false
	return m.startRun(m.modalChoice == 1)
}

// startRun executes the full scaffold (pre → render → post) for real,
// streaming hook output into the right pane. skip runs the scaffold
// with hooks disabled.
func (m hooksModel) startRun(skip bool) (hooksModel, tea.Cmd) {
	m.running = true
	m.runningAll = true
	m.modalOpen = false
	m.didRun = true
	m.focus = "view"
	m.output = ""
	if skip {
		m.output = "Skipping hooks (declined).\n"
	} else {
		m.output = "Running hooks...\n\n"
	}
	m.viewport.SetContent(m.output)
	m.viewport.GotoBottom()

	ch := make(chan string, 64)
	doneCh := make(chan error, 1)
	m.stream = ch
	m.doneCh = doneCh

	go func() {
		opts := template.HookOptions{PrintCommands: true, Verbose: m.verbose}
		if skip {
			opts.NoHooks = true
		} else {
			opts.Output = &chanWriter{ch: ch}
		}
		doneCh <- m.tpl.RenderToWithPost(m.ctx, m.dest, m.resolved, opts)
		close(ch)
	}()
	return m, m.listen()
}

// startSingleRun executes just the hook at index idx (inline command or
// script file) and streams its command + output into the right pane. It
// is an isolated preview: it does not scaffold the project and does not
// set didRun, so runNew still performs the real render afterwards.
func (m hooksModel) startSingleRun(idx int) (hooksModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.hooks) {
		return m, nil
	}
	m.running = true
	m.runningAll = false
	m.focus = "view"
	m.output = ""
	m.viewport.SetContent(m.output)
	m.viewport.GotoBottom()

	ch := make(chan string, 64)
	doneCh := make(chan error, 1)
	m.stream = ch
	m.doneCh = doneCh
	h := m.hooks[idx]

	go func() {
		opts := template.HookOptions{PrintCommands: true, Verbose: m.verbose}
		opts.Output = &chanWriter{ch: ch}
		doneCh <- template.RunSingleHook(m.ctx, m.tpl, m.resolved, m.dest, h, opts)
		close(ch)
	}()
	return m, m.listen()
}

// listen resumes reading streamed hook output until the run finishes.
func (m hooksModel) listen() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-m.stream
		if !ok {
			var err error
			if m.doneCh != nil {
				err = <-m.doneCh
			}
			return runDoneMsg{err: err}
		}
		return runLineMsg{text: line}
	}
}

func (m hooksModel) appBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(tuiAccent)),
	)
}

func (m hooksModel) appBoundaryViewFoot(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		lipgloss.NewStyle().PaddingRight(1).Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(tuiAccent)),
	)
}

func (m hooksModel) resize(width, height int) hooksModel {
	m.width = width
	m.height = height
	listH := max(height-7, 1)
	m.list.SetSize(42, listH)
	viewW := max(width-42-3, 20)
	m.viewport = viewport.New(viewport.WithWidth(viewW), viewport.WithHeight(listH))
	if !m.running && m.output == "" {
		m.viewport.SetContent(m.selectedHookContent(m.list.Index()))
		m.viewport.GotoTop()
	} else {
		m.viewport.SetContent(m.output)
	}
	return m
}

func (m hooksModel) view() tea.View {
	s := m.styles
	title := gradientText("Spin  Hooks Review — "+m.name, tuiPink, tuiAccent)
	header := m.appBoundaryView(title)

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tuiAccent).
		Padding(1, 0)
	if m.focus == "list" && !m.modalOpen {
		listStyle = listStyle.BorderForeground(tuiBrightRed)
	}
	listBox := listStyle.
		Width(42 + 2).
		Height(max(m.viewport.Height(), 1) + 2).
		Render(m.list.View())

	viewStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tuiAccent).
		Padding(1,0)

	if m.focus == "view" && !m.modalOpen {
		viewStyle = viewStyle.BorderForeground(tuiBrightYellow)
	}
	viewBox := viewStyle.
		Width(max(m.viewport.Width(), 20) + 2).
		Height(max(m.viewport.Height(), 1) + 3).
		Render(m.viewport.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, listBox, " ", viewBox)

	var footer string
	switch {
	case m.modalOpen:
		footer = "←/→ toggle • enter submit • y Run • n Skip"
	case m.running:
		if m.runningAll {
			footer = "running all hooks…  (q/esc quit)"
		} else {
			footer = "running hook…  (q/esc quit)"
		}
	case m.output != "":
		if m.didRun {
			footer = "press q to exit • R run all again"
		} else {
			footer = "press q to exit"
		}
	default:
		footer = "enter run hook • R run all • ←/→ focus • q/esc quit"
	}
	footerView := m.appBoundaryViewFoot(footer)

	inner := header + "\n" + body + "\n" + footerView
	if m.modalOpen {
		base := s.Base.Render(inner)
		modal := m.modalBox()
		canvas := lipgloss.NewCanvas(m.width, m.height)
		boxW := lipgloss.Width(modal)
		boxH := lipgloss.Height(modal)
		x := max((m.width-boxW)/2, 0)
		y := max((m.height-boxH)/2, 0)
		// The Compositor flattens the layer tree and respects each
		// layer's x/y offset, so the modal box draws centered on top
		// of the hooks view.
		comp := lipgloss.NewCompositor(
			lipgloss.NewLayer(base),
			lipgloss.NewLayer(modal).X(x).Y(y).Z(1),
		)
		canvas.Compose(comp)
		v := tea.NewView(canvas.Render())
		v.AltScreen = true
		return v
	}
	v := tea.NewView(s.Base.Render(inner))
	v.AltScreen = true
	return v
}

// modalBox renders the Run/Skip confirmation dialog content (the box
// itself). The caller composites it over the hooks view using a canvas
// and a foreground layer so it appears centered and on top.
func (m hooksModel) modalBox() string {
	var b strings.Builder
	name := m.tpl.Name
	if m.tpl.SpinToml != nil && m.tpl.SpinToml.Name != "" {
		name = m.tpl.SpinToml.Name
	}
	fmt.Fprintf(&b, "Run %q hooks?\n", name)
	source := m.tpl.Repo
	if source == "" {
		source = m.tpl.Source
	}
	fmt.Fprintf(&b, "Source: %s\n", source)
	for _, h := range m.hooks {
		if h.IsFile {
			dir := "_pre"
			if h.Phase == "post" {
				dir = "_post"
			}
			fmt.Fprintf(&b, "  %s/ files:\n", dir)
			fmt.Fprintf(&b, "    - %s\n", filepath.Base(h.File))
		} else {
			fmt.Fprintf(&b, "  [[%s]] run = %q\n", h.Phase, h.Run)
		}
	}
	b.WriteString("Only run hooks from templates you trust.\n")
	b.WriteString("\n")

	runStyle := lipgloss.NewStyle()
	skipStyle := lipgloss.NewStyle()
	if m.modalChoice == 0 {
		runStyle = runStyle.Foreground(tuiBrightRed).Bold(true)
	} else {
		skipStyle = skipStyle.Foreground(tuiBrightYellow).Bold(true)
	}
	fmt.Fprintf(&b, "   %s      %s\n", runStyle.Render("Run"), skipStyle.Render("Skip"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tuiAccent).
		Padding(1, 3)
	return boxStyle.Render(b.String())
}

var _ io.Writer = (*chanWriter)(nil)
