package cmd

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/N1xev/spin/internal/params"
	"github.com/N1xev/spin/internal/template"
)

var (
	tuiAccent       = lipgloss.Color("99")
	tuiPink         = lipgloss.Color("212")
	tuiRed          = lipgloss.Color("9")
	tuiBrightRed    = lipgloss.Color("#FF5555")
	tuiBrightYellow = lipgloss.Color("#F1FA8C")
)

type tuiStyles struct {
	Base, HeaderText, ErrorHeaderText, Status, StatusHeader lipgloss.Style
}

func newTUIStyles() *tuiStyles {
	return &tuiStyles{
		Base: lipgloss.NewStyle().Padding(1, 4, 0, 1),
		HeaderText: lipgloss.NewStyle().
			Foreground(tuiAccent).Bold(true).Padding(0, 1, 0, 2),
		ErrorHeaderText: lipgloss.NewStyle().
			Foreground(tuiRed).Bold(true).Padding(0, 1, 0, 2),
		Status: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tuiAccent).
			PaddingLeft(1).MarginTop(1),
		StatusHeader: lipgloss.NewStyle().
			Foreground(tuiAccent).Bold(true),
	}
}

type newTUIModel struct {
	styles *tuiStyles
	form   *huh.Form
	width  int
	tpl    *template.Template
	params []params.Param
}

func newNewTUIModel(tpl *template.Template, values map[string]any) (newTUIModel, error) {
	m := newTUIModel{tpl: tpl, styles: newTUIStyles(), width: termWidth()}
	ps, err := params.Parse(tpl.SpinToml.Params)
	if err != nil {
		return m, err
	}
	params.SetDefaults(ps, values)
	// Seed builtins (name, project_name) and any other supplied values
	// onto params whose name matches, so the form opens with them
	// pre-filled -- matching the non-TTY ResolveForm behaviour. Guard
	// against empty strings: an empty value must never clobber a
	// param's own default.
	for _, p := range ps {
		if v, ok := values[p.Name()]; ok {
			if s, ok := v.(string); ok && s == "" {
				continue
			}
			p.Apply(params.FromAny(v))
		}
	}
	m.params = ps
	var fields []huh.Field
	for _, p := range ps {
		fields = append(fields, p.HuhField(values))
	}
	m.form = huh.NewForm(huh.NewGroup(fields...)).
		WithWidth(min(m.width/2, 60)).
		WithShowHelp(false).
		WithShowErrors(false)
	return m, nil
}

func (m newTUIModel) Init() tea.Cmd { return m.form.Init() }

func (m newTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s := m.styles
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = min(msg.Width, termWidth()) - s.Base.GetHorizontalFrameSize()
		m.form = m.form.WithWidth(min(m.width/2, 60)).
			WithHeight(msg.Height - 8)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Interrupt
		case "esc", "q":
			return m, tea.Quit
		}
	}
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	if m.form.State == huh.StateCompleted {
		return m, tea.Quit
	}
	return m, cmd
}

func (m newTUIModel) View() tea.View {
	s := m.styles

	title := gradientText("Spin  Create Project — "+m.tpl.Name, tuiPink, tuiAccent)

	v := strings.TrimSuffix(m.form.View(), "\n\n")
	form := lipgloss.NewStyle().Margin(1, 0).Render(v)

	status := m.statusView(form)

	errors := m.form.Errors()
	header := m.appBoundaryView(title)
	if len(errors) > 0 {
		header = m.appErrorBoundaryView(errorView(errors))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Left, form, status)
	footer := m.appBoundaryViewFoot(m.form.WithWidth(m.width - 10).Help().ShortHelpView(m.form.KeyBinds()))
	m.form = m.form.WithWidth(min(m.width/2, 60))
	if len(errors) > 0 {
		footer = m.appErrorBoundaryView("")
	}
	return tea.NewView(s.Base.Render(header + "\n" + body + "\n\n" + footer))
}

func (m newTUIModel) statusView(form string) string {
	s := m.styles
	if m.width-lipgloss.Width(form) < 45 {
		return ""
	}
	thdr := func(text string, c color.Color) string {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(text)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", thdr("Template", lipgloss.Color("14")))
	fmt.Fprintf(&b, "%s\n", m.tpl.Name)
	if m.tpl.SpinToml != nil {
		if m.tpl.SpinToml.Description != "" {
			fmt.Fprintf(&b, "%s\n", m.tpl.SpinToml.Description)
		}
		if m.tpl.SpinToml.Language != "" {
			fmt.Fprintf(&b, "Language: %s\n", m.tpl.SpinToml.Language)
		}
		if m.tpl.SpinToml.Type != "" {
			fmt.Fprintf(&b, "Type: %s\n", m.tpl.SpinToml.Type)
		}
		hooks := len(m.tpl.SpinToml.Pre) + len(m.tpl.SpinToml.Post)
		if hooks > 0 {
			fmt.Fprintf(&b, "\n%s\n", thdr("Terminal", tuiBrightRed))
			for _, pre := range m.tpl.SpinToml.Pre {
				fmt.Fprintf(&b, "  pre: %s\n", pre.Run)
			}
			for _, post := range m.tpl.SpinToml.Post {
				fmt.Fprintf(&b, "  post: %s\n", post.Run)
			}
		}
	}
	fmt.Fprintf(&b, "\n%s\n", thdr("Params", tuiBrightYellow))
	for _, p := range m.params {
		if val := paramDisplay(p); val != "" {
			fmt.Fprintf(&b, "  %s: %s\n", p.Name(), val)
		}
	}
	const statusWidth = 50
	statusMarginLeft := max(m.width-statusWidth-lipgloss.Width(form)-s.Status.GetMarginRight(), 0)
	return s.Status.
		Width(statusWidth).
		Height(lipgloss.Height(form)).
		MarginLeft(statusMarginLeft).
		Render(b.String())
}

func errorView(errs []error) string {
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Error())
	}
	return b.String()
}

func paramDisplay(p params.Param) string {
	v := p.Value()
	switch v.Kind {
	case params.TypeText, params.TypeTextarea, params.TypeSelect, params.TypeSecret:
		return v.String
	case params.TypeNumber:
		return fmt.Sprintf("%d", v.Int)
	case params.TypeBool:
		if v.Bool {
			return "Yes"
		}
		return "No"
	case params.TypeMultiSelect:
		return strings.Join(v.List, ", ")
	case params.TypePath:
		return v.Path
	}
	return ""
}

func gradientText(s string, from, to color.Color) string {
	if len(s) == 0 {
		return ""
	}
	colors := lipgloss.Blend1D(len(s), from, to)
	var b strings.Builder
	for i, r := range s {
		b.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Bold(true).Render(string(r)))
	}
	return b.String()
}

func (m newTUIModel) appBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(tuiAccent)),
	)
}

func (m newTUIModel) appErrorBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(tuiRed)),
	)
}

func (m newTUIModel) appBoundaryViewFoot(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		lipgloss.NewStyle().PaddingRight(1).Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(tuiAccent)),
	)
}

func runNewTUI(tpl *template.Template, values map[string]any) (map[string]any, error) {
	m, err := newNewTUIModel(tpl, values)
	if err != nil {
		return nil, err
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, param := range m.params {
		out[param.Name()] = template.UnwrapValue(param.Value())
	}
	// Copy through any caller-supplied/builtin keys that aren't
	// backed by a param (e.g. project_name). Mirrors
	// Template.ResolveForm so the TTY and non-TTY paths produce the
	// same value map.
	for k, v := range values {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out, nil
}
