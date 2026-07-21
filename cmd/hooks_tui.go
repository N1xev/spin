package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

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

const (
	hooksListContentW = 42
	hooksListBorderW  = 2 // rounded border left + right
	hooksPaneGap      = 1 // column between list and view boxes
	hooksViewBorderW  = 2 // rounded border left + right
	hooksViewMinW     = 20
)

// hooksBodyOverhead is horizontal space in the hooks body layout taken by
// everything except the view pane's content width: list content, list
// borders, the inter-pane gap, and view borders.
const hooksBodyOverhead = hooksListContentW + hooksListBorderW + hooksPaneGap + hooksViewBorderW

func hooksViewContentW(totalW int) int {
	return max(totalW-hooksBodyOverhead, hooksViewMinW)
}

const hookHintShort = "R run all + scaffold • ←/→ focus • q/esc quit"

// wrapForView hard-wraps s to the viewport width. It uses ansi.Hardwrap
// so ANSI styling is preserved and long unbroken tokens (inline hook
// commands, file contents) are broken across lines instead of
// overflowing the pane edge.
func wrapForView(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Hardwrap(s, width, false)
}

// hooksModel renders the interactive hook review screen. The left pane
// lists every hook the template will run; the right pane streams the
// live output when the hooks execute. Pressing R opens a centered
// Run/Skip/Cancel modal (replacing the CLI trust prompt); confirming
// Run or Skip executes the full scaffold (pre → render → post) and
// streams its output into the right pane. The modal only asks whether
// to run the hooks, never repeats the hook list.
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

	running bool
	output  string
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
	l := list.New(listItems, delegate, hooksListContentW, listH)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.SetSize(hooksListContentW, listH)
	if len(listItems) > 0 {
		l.Select(0)
	}

	viewW := hooksViewContentW(width)
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
	m.viewport.SetContent(wrapForView(m.selectedHookContent(m.list.Index()), m.viewport.Width()))
	m.viewport.GotoTop()
	if noHooks {
		m.autoStart = 2
	} else if yes {
		m.autoStart = 1
	}
	return m
}

// selectedHookContent builds the right-pane content shown while the
// user is reviewing hooks: the inline [[pre]]/[[post]] command or the
// contents of the _pre/_post script file for the hook at idx. It is
// replaced by streamed command + output once the scaffold runs.
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
		m.viewport.SetContent(wrapForView(m.output, m.viewport.Width()))
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
			m.output += "\n"
			m.output += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(
				fmt.Sprintf("INFO created %s at %s", m.name, m.dest))
			m.output += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
				fmt.Sprintf("cd %s", m.dest))
		}
		m.viewport.SetContent(wrapForView(m.output, m.viewport.Width()))
		m.viewport.GotoBottom()
		return m, nil
	case tea.KeyPressMsg:
		if m.modalOpen {
			switch msg.String() {
			case "left", "h", "tab":
				m.modalChoice--
				if m.modalChoice < 0 {
					m.modalChoice = 2
				}
				return m, nil
			case "right", "l":
				m.modalChoice++
				if m.modalChoice > 2 {
					m.modalChoice = 0
				}
				return m, nil
			case "enter":
				return m.submitModal()
			case "y":
				m.modalChoice = 0
				return m.submitModal()
			case "n":
				m.modalChoice = 1
				return m.submitModal()
			case "c":
				m.modalChoice = 2
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
				m.viewport.SetContent(wrapForView(m.selectedHookContent(m.list.Index()), m.viewport.Width()))
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
			m.viewport.SetContent(wrapForView(m.selectedHookContent(m.list.Index()), m.viewport.Width()))
			m.viewport.GotoTop()
		}
		return m, cmd
	}
	return m, nil
}

// submitModal resolves the current modal choice. 0 = Run (full
// scaffold with hooks), 1 = Skip (scaffold without hooks), 2 = Cancel
// (dismiss the modal and do nothing).
func (m hooksModel) submitModal() (hooksModel, tea.Cmd) {
	m.modalOpen = false
	switch m.modalChoice {
	case 1:
		return m.startRun(true)
	case 2:
		return m, nil
	default:
		return m.startRun(false)
	}
}

// startRun executes the full scaffold (pre → render → post) for real,
// streaming hook output into the right pane. skip runs the scaffold
// with hooks disabled.
func (m hooksModel) startRun(skip bool) (hooksModel, tea.Cmd) {
	m.running = true
	m.modalOpen = false
	m.didRun = true
	m.focus = "view"
	m.output = ""
	if skip {
		m.output = "Skipping hooks (declined).\n"
	} else {
		m.output = "Running hooks...\n\n"
	}
	m.viewport.SetContent(wrapForView(m.output, m.viewport.Width()))
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
			opts.Output = channelHookOutput(ch)
			opts.StepStart = func(kind, cmd string) {
				ch <- hookStepHeader(kind, cmd)
			}
		}
		doneCh <- m.tpl.RenderToWithPost(m.ctx, m.dest, m.resolved, opts)
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
	m.list.SetSize(hooksListContentW, listH)
	viewW := hooksViewContentW(width)
	m.viewport = viewport.New(viewport.WithWidth(viewW), viewport.WithHeight(listH))
	if !m.running && m.output == "" {
		m.viewport.SetContent(wrapForView(m.selectedHookContent(m.list.Index()), m.viewport.Width()))
		m.viewport.GotoTop()
	} else {
		m.viewport.SetContent(wrapForView(m.output, m.viewport.Width()))
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
		Width(hooksListContentW + hooksListBorderW).
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
		Width(max(m.viewport.Width(), hooksViewMinW) + hooksViewBorderW).
		Height(max(m.viewport.Height(), 1) + 3).
		Render(m.viewport.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, listBox, " ", viewBox)

	var footer string
	switch {
	case m.modalOpen:
		footer = "←/→ toggle • enter submit • y Run • n Skip • c Cancel"
	case m.running:
		footer = "running scaffold…  (q/esc quit)"
	case m.output != "":
		if m.didRun {
			footer = "press q to exit • R run all again"
		} else {
			footer = "press q to exit"
		}
	default:
		footer = hookHintShort
	}
	footerView := m.appBoundaryViewFoot(footer)

	inner := header + "\n" + body + "\n" + footerView
	if m.modalOpen {
		// Size the canvas to the rendered base, not to m.width. m.width
		// is the inner width (the Base style's 5-column horizontal frame
		// is already subtracted in newTUIStyles), but base = s.Base.Render(inner)
		// adds that frame back. A m.width-sized canvas would clip base's
		// right edge, erasing the view pane's right border on the rows
		// the modal overlaps. Matching the canvas to base keeps every
		// base cell (the border included) and lets the modal layer draw
		// centered on top without erasing anything outside its own box.
		base := s.Base.Render(inner)
		modal := m.modalBox()
		canvas := lipgloss.NewCanvas(lipgloss.Width(base), lipgloss.Height(base))
		cw, ch := canvas.Width(), canvas.Height()
		boxW := lipgloss.Width(modal)
		boxH := lipgloss.Height(modal)
		x := max((cw-boxW)/2, 0)
		y := max((ch-boxH)/2, 0)
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

// modalBox renders the Run/Skip/Cancel confirmation dialog content (the
// box itself). The caller composites it over the hooks view using a
// canvas and a foreground layer so it appears centered and on top. The
// dialog only asks whether to run the template's hooks; it does not
// repeat the hook list (that is already visible in the left pane).
func (m hooksModel) modalBox() string {
	var b strings.Builder
	name := m.tpl.Name
	if m.tpl.SpinToml != nil && m.tpl.SpinToml.Name != "" {
		name = m.tpl.SpinToml.Name
	}
	fmt.Fprintf(&b, "Run %q hooks?\n\n", name)
	fmt.Fprintf(&b, "Only run hooks from templates you trust.\n")

	runStyle := lipgloss.NewStyle()
	skipStyle := lipgloss.NewStyle()
	cancelStyle := lipgloss.NewStyle()
	switch m.modalChoice {
	case 0:
		runStyle = runStyle.Foreground(tuiBrightRed).Bold(true)
	case 1:
		skipStyle = skipStyle.Foreground(tuiBrightYellow).Bold(true)
	default:
		cancelStyle = cancelStyle.Foreground(lipgloss.Color("245")).Bold(true)
	}
	fmt.Fprintf(&b, "\n  %s    %s    %s\n",
		runStyle.Render("Run"),
		skipStyle.Render("Skip"),
		cancelStyle.Render("Cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tuiAccent).
		Padding(1, 3)
	return boxStyle.Render(b.String())
}

