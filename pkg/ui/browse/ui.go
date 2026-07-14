package browse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
)

type Options struct {
	Reference     ingest.Reference
	IncludeHidden bool
}

type UI struct {
	model *browseModel
}

func New(options Options) (*UI, error) {
	ref := options.Reference
	if ref.Provider == "" {
		ref.Provider = "path"
		if ref.Path == "" {
			ref.Path = "./"
		}
	}
	ref.Symbol = ""

	model, err := newBrowseModelFromReference(ref, options.IncludeHidden)
	if err != nil {
		return nil, err
	}
	return &UI{model: model}, nil
}

func (ui *UI) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	final, err := tea.NewProgram(ui.model, tea.WithContext(ctx), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if err != nil {
		return err
	}
	out, ok := final.(*browseModel)
	if !ok {
		return nil
	}
	return out.err
}

type browseFocus int

const (
	browseFocusList browseFocus = iota
	browseFocusPreview
)

const splitLayoutMinWidth = 110

type browseItemKind int

const (
	browseItemParent browseItemKind = iota
	browseItemDir
	browseItemFile
	browseItemSymbol
	browseItemInfo
)

type browseItem struct {
	kind      browseItemKind
	title     string
	desc      string
	targetRel string
	targetRef string
	symbolRef string
}

func (i browseItem) Title() string       { return i.title }
func (i browseItem) Description() string { return i.desc }
func (i browseItem) FilterValue() string { return i.title + " " + i.desc }

type docLoadedMsg struct {
	ref      string
	markdown string
}

type docDebounceMsg struct {
	tag uint64
}

type browseKeys struct {
	Open        key.Binding
	Parent      key.Binding
	ToggleAll   key.Binding
	ToggleHelp  key.Binding
	Refresh     key.Binding
	ToggleFocus key.Binding
	Quit        key.Binding
	PreviewUp   key.Binding
	PreviewDown key.Binding
	ListTop     key.Binding
	ListBottom  key.Binding
}

type browseNavState struct {
	mode         string
	currentRel   string
	providerRef  ingest.Reference
	providerDir  string
	openedSymbol string
	focus        browseFocus
	listIndex    int
}

func newBrowseKeys() browseKeys {
	return browseKeys{
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "push/open"),
		),
		Parent: key.NewBinding(
			key.WithKeys("h", "backspace", "left", "esc"),
			key.WithHelp("h/esc", "pop/back"),
		),
		ToggleAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle hidden"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		ToggleFocus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "focus list/preview"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		PreviewUp: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "preview up"),
		),
		PreviewDown: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/down", "preview down"),
		),
		ListTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		ListBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
	}
}

func (k browseKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Parent, k.ToggleFocus, k.ToggleAll, k.ToggleHelp, k.Quit}
}

func (k browseKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.Parent, k.Refresh, k.ToggleAll},
		{k.ToggleFocus, k.PreviewUp, k.PreviewDown},
		{k.ListTop, k.ListBottom, k.ToggleHelp, k.Quit},
	}
}

type browseModel struct {
	rootDir       string
	resolver      *ingest.Resolver
	currentRel    string
	mode          string
	providerRef   ingest.Reference
	providerDir   string
	openedSymbol  string
	navStack      []browseNavState
	includeHidden bool
	focus         browseFocus
	showFullHelp  bool
	showSplit     bool
	keys          browseKeys

	list    list.Model
	preview viewport.Model
	help    help.Model

	docCache    map[string]string
	loadingDocs map[string]bool
	docDebounce uint64
	renderCache map[string]string
	renderWrap  int
	renderer    *glamour.TermRenderer
	err         error

	width        int
	height       int
	listWidth    int
	previewWidth int
	bodyHeight   int
}

func (m *browseModel) refs() *ingest.Resolver {
	if m != nil && m.resolver != nil {
		return m.resolver
	}
	return ingest.NewResolver("")
}

func newBrowseModelFromReference(ref ingest.Reference, includeHidden bool) (*browseModel, error) {
	if ref.Provider == "path" {
		rootDir, currentRel, err := browseScopeFromReference(ref)
		if err != nil {
			return nil, err
		}
		return newBrowseModel(rootDir, currentRel, includeHidden)
	}

	scope, ok, err := ingest.NewResolver(".").ResolveScopeTarget(ref)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("browse not supported for provider %q", ref.Provider)
	}

	model, err := newBrowseModel(".", ".", includeHidden)
	if err != nil {
		return nil, err
	}
	model.mode = "provider"
	model.providerRef = ingest.Reference{
		Provider: ref.Provider,
		Path:     strings.Trim(ref.Path, "/"),
	}
	model.providerDir = scope.Dir

	if err := model.reload(); err != nil {
		return nil, err
	}
	return model, nil
}

func newBrowseModel(rootDir, currentRel string, includeHidden bool) (*browseModel, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	model := &browseModel{
		rootDir:       absRoot,
		resolver:      ingest.NewResolver(absRoot),
		currentRel:    currentRel,
		mode:          "path",
		includeHidden: includeHidden,
		focus:         browseFocusList,
		keys:          newBrowseKeys(),
		docCache:      map[string]string{},
		loadingDocs:   map[string]bool{},
		renderCache:   map[string]string{},
	}
	model.help.ShowAll = false

	model.list = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	model.list.Title = "rft browse"
	model.list.SetShowHelp(false)
	model.list.SetShowStatusBar(false)
	model.list.SetShowPagination(false)
	model.list.SetFilteringEnabled(false)
	model.list.DisableQuitKeybindings()

	model.preview = viewport.New(0, 0)
	model.preview.SetContent("Select an entry to inspect details.")

	if err := model.setCurrentRel(currentRel); err != nil {
		return nil, err
	}
	if err := model.reload(); err != nil {
		return nil, err
	}
	return model, nil
}

func (m *browseModel) Init() tea.Cmd {
	return m.scheduleDocLoadCmd()
}

func (m *browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.renderCache = map[string]string{}
		m.updatePreviewForSelection()
		return m, m.scheduleDocLoadCmd()
	case docDebounceMsg:
		if msg.tag != m.docDebounce {
			return m, nil
		}
		return m, m.ensureSelectedDocLoadedCmd()
	case docLoadedMsg:
		delete(m.loadingDocs, msg.ref)
		m.docCache[msg.ref] = msg.markdown
		if m.activeSymbolRef() == msg.ref {
			m.updatePreviewForSelection()
		}
		return m, m.scheduleDocLoadCmd()
	case tea.MouseMsg:
		cmd := m.handleMouse(msg)
		return m, tea.Batch(cmd, m.scheduleDocLoadCmd())
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.showFullHelp = !m.showFullHelp
			m.help.ShowAll = m.showFullHelp
			m.resize()
			m.updatePreviewForSelection()
			return m, m.scheduleDocLoadCmd()
		case key.Matches(msg, m.keys.ToggleAll):
			m.includeHidden = !m.includeHidden
			if err := m.reload(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.scheduleDocLoadCmd()
		case key.Matches(msg, m.keys.Refresh):
			if err := m.reload(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.scheduleDocLoadCmd()
		case key.Matches(msg, m.keys.Parent):
			if ok, err := m.popState(); err != nil {
				m.err = err
				return m, tea.Quit
			} else if ok {
				return m, m.scheduleDocLoadCmd()
			}

			if m.focus == browseFocusPreview && m.openedSymbol == "" {
				m.focus = browseFocusList
				m.updatePreviewForSelection()
				return m, m.scheduleDocLoadCmd()
			}
			if err := m.goParent(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.scheduleDocLoadCmd()
		case key.Matches(msg, m.keys.ToggleFocus):
			if m.focus == browseFocusList {
				m.focus = browseFocusPreview
			} else {
				m.focus = browseFocusList
			}
			m.updatePreviewForSelection()
			return m, m.scheduleDocLoadCmd()
		case key.Matches(msg, m.keys.Open):
			if err := m.activateSelection(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.scheduleDocLoadCmd()
		}

		if m.focus == browseFocusPreview {
			switch {
			case key.Matches(msg, m.keys.PreviewUp):
				m.preview.ScrollUp(1)
				return m, nil
			case key.Matches(msg, m.keys.PreviewDown):
				m.preview.ScrollDown(1)
				return m, nil
			}
		}
	}

	if m.focus == browseFocusList {
		prevIndex := m.list.Index()
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if m.list.Index() != prevIndex {
			m.updatePreviewForSelection()
			return m, tea.Batch(cmd, m.scheduleDocLoadCmd())
		}
		return m, cmd
	}
	return m, nil
}

func (m *browseModel) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	focusLabel := "list"
	if m.focus == browseFocusPreview {
		focusLabel = "preview"
	}
	current := m.currentScopeRef()
	mode := "hidden:off"
	if m.includeHidden {
		mode = "hidden:on"
	}
	layout := "single"
	if m.showSplit {
		layout = "split"
	}

	m.list.Title = fmt.Sprintf("%s  (%s)", current, focusLabel)

	var body string
	if m.showSplit {
		leftStyle := lipgloss.NewStyle().Width(m.listWidth).Height(m.bodyHeight)
		rightStyle := lipgloss.NewStyle().Width(m.previewWidth).Height(m.bodyHeight).PaddingLeft(1)
		if m.focus == browseFocusList {
			leftStyle = leftStyle.Border(lipgloss.NormalBorder(), false, true, false, false)
		}
		if m.focus == browseFocusPreview {
			rightStyle = rightStyle.Border(lipgloss.NormalBorder(), false, false, false, true)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftStyle.Render(m.list.View()), rightStyle.Render(m.preview.View()))
	} else {
		panelStyle := lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight)
		if m.focus == browseFocusPreview || m.openedSymbol != "" {
			panelStyle = panelStyle.Border(lipgloss.NormalBorder(), false, false, false, true)
			body = panelStyle.Render(m.preview.View())
		} else {
			panelStyle = panelStyle.Border(lipgloss.NormalBorder(), false, true, false, false)
			body = panelStyle.Render(m.list.View())
		}
	}
	status := fmt.Sprintf("root:%s  scope:%s  %s  layout:%s  stack:%d", m.scopeRoot(), current, mode, layout, len(m.navStack))

	return body + "\n" + status + "\n" + m.help.View(m.keys)
}

func (m *browseModel) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	helpHeight := 1
	if m.showFullHelp {
		helpHeight = 3
	}
	m.bodyHeight = m.height - helpHeight - 1
	if m.bodyHeight < 4 {
		m.bodyHeight = 4
	}

	m.showSplit = m.width >= splitLayoutMinWidth
	if m.showSplit {
		m.listWidth = m.width / 2
		if m.listWidth < 32 {
			m.listWidth = 32
		}
		m.previewWidth = m.width - m.listWidth
		if m.previewWidth < 24 {
			m.previewWidth = 24
			m.listWidth = m.width - m.previewWidth
			if m.listWidth < 20 {
				m.listWidth = 20
			}
		}
		if m.listWidth+m.previewWidth > m.width {
			m.previewWidth = m.width - m.listWidth
			if m.previewWidth < 0 {
				m.previewWidth = 0
			}
		}
	} else {
		m.listWidth = m.width
		m.previewWidth = m.width
	}

	m.list.SetSize(m.listWidth, m.bodyHeight)
	m.preview.Width = m.previewWidth
	m.preview.Height = m.bodyHeight
}

func (m *browseModel) reload() error {
	items, err := m.buildItems()
	if err != nil {
		return err
	}
	m.list.SetItems(items)
	if len(items) == 0 {
		m.preview.SetContent("No entries")
		return nil
	}
	if m.list.Index() >= len(items) {
		m.list.Select(len(items) - 1)
	}
	m.updatePreviewForSelection()
	return nil
}

func (m *browseModel) buildItems() ([]list.Item, error) {
	if m.mode == "provider" {
		return m.buildProviderItems()
	}

	abs := m.currentAbsPath()
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, 32)
	if m.currentRel != "." {
		parent := parentRel(m.currentRel)
		items = append(items, browseItem{
			kind:      browseItemParent,
			title:     "..",
			desc:      "parent",
			targetRel: parent,
			targetRef: m.scopeRefForPathRel(parent),
		})
	}

	if st.IsDir() {
		entries, err := os.ReadDir(abs)
		if err != nil {
			return nil, err
		}

		dirs := make([]browseItem, 0, len(entries))
		modules := make([]browseItem, 0, len(entries))
		onlyGoModules := true
		hasModules := false
		for _, entry := range entries {
			name := entry.Name()
			if !m.includeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			target := joinRelPath(m.currentRel, name)
			if entry.IsDir() {
				dirs = append(dirs, browseItem{
					kind:      browseItemDir,
					title:     name + "/",
					desc:      "subpackage",
					targetRel: target,
					targetRef: m.scopeRefForPathRel(target),
				})
				continue
			}

			lang, ok := ingest.LanguageForFile(name)
			if !ok {
				continue
			}
			hasModules = true
			if !ingest.LanguageUsesDirectoryModule(lang) {
				onlyGoModules = false
				modules = append(modules, browseItem{
					kind:      browseItemFile,
					title:     name,
					desc:      fmt.Sprintf("%s module", lang),
					targetRel: target,
					targetRef: m.scopeRefForPathRel(target),
				})
			}
		}

		slices.SortFunc(dirs, func(a, b browseItem) int { return strings.Compare(a.title, b.title) })
		for _, it := range dirs {
			items = append(items, it)
		}
		slices.SortFunc(modules, func(a, b browseItem) int { return strings.Compare(a.title, b.title) })
		for _, it := range modules {
			items = append(items, it)
		}

		if hasModules && !onlyGoModules {
			if len(items) == 0 {
				items = append(items, browseItem{
					kind:  browseItemInfo,
					title: "(empty)",
					desc:  "no files or symbols",
				})
			}
			return items, nil
		}
	}

	symbols, err := m.symbolItems()
	if err != nil {
		return nil, err
	}
	items = append(items, symbols...)

	if len(items) == 0 {
		items = append(items, browseItem{
			kind:  browseItemInfo,
			title: "(empty)",
			desc:  "no files or symbols",
		})
	}
	return items, nil
}

func (m *browseModel) buildProviderItems() ([]list.Item, error) {
	scope, ok, err := m.refs().ResolveScopeTarget(m.providerRef)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("provider scope navigation not supported for %q", m.providerRef.Provider)
	}
	m.providerDir = scope.Dir

	items := make([]list.Item, 0, 32)
	parent := refpkg.ParentProviderPath(m.providerRef.Path)
	if parent != m.providerRef.Path {
		items = append(items, browseItem{
			kind:      browseItemParent,
			title:     "..",
			desc:      "parent package",
			targetRef: ingest.Reference{Provider: m.providerRef.Provider, Path: parent}.String(),
		})
	}

	allowChildren := true
	if scope.CanDescend != nil {
		allowChildren = *scope.CanDescend
	}
	if allowChildren {
		if children, ok, err := m.refs().ResolveScopeChildren(m.providerRef, m.includeHidden); err != nil {
			return nil, err
		} else if ok {
			for _, child := range children {
				items = append(items, browseItemForProviderChild(child))
			}
		} else {
			entries, err := os.ReadDir(m.providerDir)
			if err != nil {
				return nil, err
			}

			packages := make([]browseItem, 0, len(entries))
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				if !m.includeHidden && strings.HasPrefix(name, ".") {
					continue
				}
				childPath := refpkg.JoinProviderPath(m.providerRef.Path, name)
				packages = append(packages, browseItem{
					kind:      browseItemDir,
					title:     name + "/",
					desc:      "subpackage",
					targetRef: ingest.Reference{Provider: m.providerRef.Provider, Path: childPath}.String(),
				})
			}
			slices.SortFunc(packages, func(a, b browseItem) int { return strings.Compare(a.title, b.title) })
			for _, pkg := range packages {
				items = append(items, pkg)
			}
		}
	}

	symbols, err := m.symbolItems()
	if err != nil {
		return nil, err
	}
	items = append(items, symbols...)

	if len(items) == 0 {
		items = append(items, browseItem{
			kind:  browseItemInfo,
			title: "(empty)",
			desc:  "no packages or symbols",
		})
	}
	return items, nil
}

func (m *browseModel) symbolItems() ([]list.Item, error) {
	dir, ref := m.currentListingScope()
	options := ingest.ListOptions{IncludeHidden: m.includeHidden}
	out := make([]list.Item, 0, 64)

	err := ingest.WalkSymbols(dir, ref, options, func(sym ingest.SymbolInfo) bool {
		path := strings.TrimPrefix(sym.Reference.Path, "./")
		if path == "" {
			path = "."
		}
		out = append(out, browseItem{
			kind:      browseItemSymbol,
			title:     sym.Reference.Symbol,
			desc:      fmt.Sprintf("%s [%s]", path, sym.Language),
			symbolRef: sym.Entity.Reference,
		})
		return true
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *browseModel) updatePreviewForSelection() {
	if m.openedSymbol != "" {
		m.updatePreviewForOpenedSymbol()
		return
	}

	item, ok := m.list.SelectedItem().(browseItem)
	if !ok {
		m.preview.SetContent(m.renderMarkdown("No selection"))
		return
	}

	var md strings.Builder
	write := func(format string, args ...any) {
		_, _ = fmt.Fprintf(&md, format, args...)
		md.WriteByte('\n')
	}

	write("## Scope")
	write("- **Reference:** `%s`", m.currentScopeRef())
	write("- **Root:** `%s`", m.scopeRoot())
	md.WriteByte('\n')

	write("## Entry")
	write("- **Name:** `%s`", item.title)
	write("- **Kind:** `%s`", item.kindName())

	switch item.kind {
	case browseItemParent, browseItemDir, browseItemFile:
		target := item.targetRel
		if item.targetRef != "" {
			target = item.targetRef
		}
		write("- **Target:** `%s`", target)
		md.WriteByte('\n')
		if item.kind == browseItemFile {
			write("_Press Enter to focus this file and list only its symbols._")
		} else {
			write("_Press Enter to enter._")
		}
	case browseItemSymbol:
		write("- **Reference:** `%s`", item.symbolRef)
		md.WriteByte('\n')
		if doc, ok := m.docCache[item.symbolRef]; ok {
			header := m.renderMarkdown(md.String())
			body := m.renderDocMarkdown(item.symbolRef, doc)
			combined := strings.TrimRight(header, "\n")
			if body != "" {
				combined += "\n\n" + strings.TrimLeft(body, "\n")
			}
			m.preview.SetContent(combined)
			m.preview.GotoTop()
			return
		} else if m.loadingDocs[item.symbolRef] {
			write("_Loading documentation..._")
		} else {
			write("_Loading documentation..._")
		}
	}

	m.preview.SetContent(m.renderMarkdown(md.String()))
	m.preview.GotoTop()
}

func (m *browseModel) updatePreviewForOpenedSymbol() {
	ref := m.openedSymbol
	parsed := ingest.ParseReference(ref)
	name := parsed.Symbol
	if name == "" {
		name = ref
	}

	var md strings.Builder
	write := func(format string, args ...any) {
		_, _ = fmt.Fprintf(&md, format, args...)
		md.WriteByte('\n')
	}

	write("## Symbol")
	write("- **Name:** `%s`", name)
	write("- **Reference:** `%s`", ref)
	write("- **Scope:** `%s`", m.currentScopeRef())
	md.WriteByte('\n')
	write("_Press Esc to go back._")

	if doc, ok := m.docCache[ref]; ok {
		header := m.renderMarkdown(md.String())
		body := m.renderDocMarkdown(ref, doc)
		combined := strings.TrimRight(header, "\n")
		if body != "" {
			combined += "\n\n" + strings.TrimLeft(body, "\n")
		}
		m.preview.SetContent(combined)
		m.preview.GotoTop()
		return
	}

	write("_Loading documentation..._")
	m.preview.SetContent(m.renderMarkdown(md.String()))
	m.preview.GotoTop()
}

func (m *browseModel) activateSelection() error {
	item, ok := m.list.SelectedItem().(browseItem)
	if !ok {
		return nil
	}

	switch item.kind {
	case browseItemParent, browseItemDir, browseItemFile:
		prev := m.snapshotState()
		if m.mode == "provider" {
			if item.targetRef == "" {
				return nil
			}
			ref := ingest.ParseReference(item.targetRef)
			ref.Symbol = ""
			scope, ok, err := m.refs().ResolveScopeTarget(ref)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("provider scope navigation not supported for %q", ref.Provider)
			}
			m.navStack = append(m.navStack, prev)
			m.providerRef = ref
			m.providerDir = scope.Dir
			m.openedSymbol = ""
			m.focus = browseFocusList
			m.list.Select(0)
			return m.reload()
		}
		if err := m.setCurrentRel(item.targetRel); err != nil {
			return err
		}
		m.navStack = append(m.navStack, prev)
		m.openedSymbol = ""
		m.focus = browseFocusList
		m.list.Select(0)
		return m.reload()
	case browseItemSymbol:
		if item.symbolRef == "" {
			return nil
		}
		if m.openedSymbol != item.symbolRef {
			m.navStack = append(m.navStack, m.snapshotState())
		}
		m.openedSymbol = item.symbolRef
		m.focus = browseFocusPreview
		m.updatePreviewForSelection()
	}
	return nil
}

func (m *browseModel) goParent() error {
	if m.mode == "provider" {
		parent := refpkg.ParentProviderPath(m.providerRef.Path)
		if parent == m.providerRef.Path {
			return nil
		}
		ref := m.providerRef
		ref.Path = parent
		scope, ok, err := m.refs().ResolveScopeTarget(ref)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("provider scope navigation not supported for %q", ref.Provider)
		}
		m.providerRef = ref
		m.providerDir = scope.Dir
		m.openedSymbol = ""
		m.focus = browseFocusList
		m.list.Select(0)
		return m.reload()
	}

	if m.currentRel == "." {
		return nil
	}
	if err := m.setCurrentRel(parentRel(m.currentRel)); err != nil {
		return err
	}
	m.openedSymbol = ""
	m.focus = browseFocusList
	m.list.Select(0)
	return m.reload()
}

func (m *browseModel) setCurrentRel(rel string) error {
	if rel == "" || rel == "." {
		m.currentRel = "."
		return nil
	}

	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) {
		return fmt.Errorf("absolute path is outside of scope: %q", rel)
	}

	abs := filepath.Join(m.rootDir, clean)
	normalized, err := filepath.Rel(m.rootDir, abs)
	if err != nil {
		return err
	}
	if normalized == ".." || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path outside root scope: %q", rel)
	}
	if normalized == "." {
		m.currentRel = "."
		return nil
	}
	m.currentRel = filepath.ToSlash(normalized)
	return nil
}

func (m *browseModel) currentAbsPath() string {
	if m.currentRel == "." {
		return m.rootDir
	}
	return filepath.Join(m.rootDir, filepath.FromSlash(m.currentRel))
}

func (m *browseModel) currentScopeRef() string {
	if m.mode == "provider" {
		return m.providerRef.String()
	}

	if m.currentRel == "." {
		return "path:./"
	}
	return "path:./" + filepath.ToSlash(m.currentRel)
}

func (m *browseModel) scopeRoot() string {
	if m.mode == "provider" && m.providerDir != "" {
		return m.providerDir
	}
	return m.rootDir
}

func (m *browseModel) currentDocScopeDir() string {
	dir, _ := m.currentListingScope()
	return dir
}

func (m *browseModel) currentListingScope() (string, string) {
	if m.mode == "provider" {
		return ".", m.currentScopeRef()
	}

	scope := ingest.ResolveInputReferenceScope(m.rootDir, m.currentAbsPath())
	return scope.Dir, scope.Reference.String()
}

func (m *browseModel) selectedSymbolRef() string {
	item, ok := m.list.SelectedItem().(browseItem)
	if !ok || item.kind != browseItemSymbol || item.symbolRef == "" {
		return ""
	}
	return item.symbolRef
}

func (m *browseModel) activeSymbolRef() string {
	if m.openedSymbol != "" {
		return m.openedSymbol
	}
	return m.selectedSymbolRef()
}

func (m *browseModel) ensureSelectedDocLoadedCmd() tea.Cmd {
	ref := m.activeSymbolRef()
	if ref == "" {
		return nil
	}
	if _, ok := m.docCache[ref]; ok {
		return nil
	}
	if len(m.loadingDocs) > 0 {
		return nil
	}
	if m.loadingDocs[ref] {
		return nil
	}

	dir := m.currentDocScopeDir()
	m.loadingDocs[ref] = true
	return func() tea.Msg {
		doc, err := ingest.DocFor(dir, ref)
		if err != nil {
			return docLoadedMsg{
				ref:      ref,
				markdown: fmt.Sprintf("Documentation unavailable: %v", err),
			}
		}
		return docLoadedMsg{
			ref:      ref,
			markdown: docToMarkdown(doc, markdownFenceLanguageForRef(ref)),
		}
	}
}

func (m *browseModel) scheduleDocLoadCmd() tea.Cmd {
	ref := m.activeSymbolRef()
	if ref == "" {
		return nil
	}
	if _, ok := m.docCache[ref]; ok {
		return nil
	}
	if len(m.loadingDocs) > 0 {
		return nil
	}
	if m.loadingDocs[ref] {
		return nil
	}

	m.docDebounce++
	tag := m.docDebounce
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return docDebounceMsg{tag: tag}
	})
}

func (m *browseModel) snapshotState() browseNavState {
	return browseNavState{
		mode:         m.mode,
		currentRel:   m.currentRel,
		providerRef:  m.providerRef,
		providerDir:  m.providerDir,
		openedSymbol: m.openedSymbol,
		focus:        m.focus,
		listIndex:    m.list.Index(),
	}
}

func (m *browseModel) popState() (bool, error) {
	if len(m.navStack) == 0 {
		return false, nil
	}

	last := m.navStack[len(m.navStack)-1]
	m.navStack = m.navStack[:len(m.navStack)-1]
	if err := m.restoreState(last); err != nil {
		return false, err
	}
	return true, nil
}

func (m *browseModel) restoreState(state browseNavState) error {
	m.mode = state.mode
	m.currentRel = state.currentRel
	m.providerRef = state.providerRef
	m.providerDir = state.providerDir
	m.openedSymbol = state.openedSymbol
	m.focus = state.focus

	if err := m.reload(); err != nil {
		return err
	}

	items := m.list.Items()
	if len(items) == 0 {
		return nil
	}
	index := state.listIndex
	if index < 0 {
		index = 0
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	m.list.Select(index)
	m.updatePreviewForSelection()
	return nil
}

func (m *browseModel) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.bodyHeight <= 0 {
		return nil
	}

	inList := m.mouseInListPane(msg.X, msg.Y)
	inPreview := m.mouseInPreviewPane(msg.X, msg.Y)

	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		if inList {
			m.focus = browseFocusList
			selectedChanged, ok := m.selectListIndexFromMouse(msg.Y)
			if ok && !selectedChanged {
				if err := m.activateSelection(); err != nil {
					m.err = err
					return tea.Quit
				}
				m.updatePreviewForSelection()
				return nil
			}
			if selectedChanged {
				m.updatePreviewForSelection()
				return nil
			}
			m.updatePreviewForSelection()
			return nil
		}
		if inPreview {
			m.focus = browseFocusPreview
			m.updatePreviewForSelection()
			return nil
		}
	}

	if inPreview {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		return cmd
	}

	if inList && msg.Action == tea.MouseActionPress {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.list.CursorUp()
			m.updatePreviewForSelection()
			return nil
		case tea.MouseButtonWheelDown:
			m.list.CursorDown()
			m.updatePreviewForSelection()
			return nil
		}
	}

	return nil
}

func (m *browseModel) mouseInListPane(x, y int) bool {
	if y < 0 || y >= m.bodyHeight {
		return false
	}
	if m.showSplit {
		return x >= 0 && x < m.listWidth
	}
	return m.focus == browseFocusList && m.openedSymbol == ""
}

func (m *browseModel) mouseInPreviewPane(x, y int) bool {
	if y < 0 || y >= m.bodyHeight {
		return false
	}
	if m.showSplit {
		return x >= m.listWidth && x < m.width
	}
	return m.focus == browseFocusPreview || m.openedSymbol != ""
}

func (m *browseModel) selectListIndexFromMouse(y int) (selectedChanged bool, clickedItem bool) {
	row := y
	if m.list.ShowTitle() {
		row--
	}
	if row < 0 {
		return false, false
	}

	itemRowHeight := 3
	offset := row / itemRowHeight
	index := m.list.Paginator.Page*m.list.Paginator.PerPage + offset

	visible := m.list.VisibleItems()
	if index < 0 || index >= len(visible) {
		return false, false
	}
	if index == m.list.Index() {
		return false, true
	}
	m.list.Select(index)
	return true, true
}

func (m *browseModel) renderMarkdown(markdown string) string {
	if markdown == "" {
		return ""
	}

	wrap := m.markdownWrapWidth()
	key := fmt.Sprintf("md:%d:%s", wrap, markdown)
	if cached, ok := m.renderCache[key]; ok {
		return cached
	}
	out := m.renderWithRenderer(wrap, markdown)
	m.renderCache[key] = out
	return out
}

func (m *browseModel) renderDocMarkdown(ref, markdown string) string {
	wrap := m.markdownWrapWidth()
	key := fmt.Sprintf("doc:%s:%d", ref, wrap)
	if cached, ok := m.renderCache[key]; ok {
		return cached
	}
	rendered := m.renderWithRenderer(wrap, markdown)
	m.renderCache[key] = rendered
	return rendered
}

func (m *browseModel) renderWithRenderer(wrap int, markdown string) string {
	if m.renderer == nil || m.renderWrap != wrap {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(wrap),
		)
		if err != nil {
			return markdown
		}
		m.renderer = renderer
		m.renderWrap = wrap
	}

	out, err := m.renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return strings.TrimRight(out, "\n")
}

func (m *browseModel) markdownWrapWidth() int {
	wrap := m.previewWidth
	if wrap <= 0 {
		wrap = 80
	}
	// Keep a small margin so wrapped markdown doesn't touch viewport edges.
	if wrap > 4 {
		wrap -= 2
	}
	return wrap
}

func (i browseItem) kindName() string {
	switch i.kind {
	case browseItemParent:
		return "parent"
	case browseItemDir:
		return "directory"
	case browseItemFile:
		return "file"
	case browseItemSymbol:
		return "symbol"
	default:
		return "info"
	}
}

func parentRel(rel string) string {
	if rel == "." || rel == "" {
		return "."
	}
	parent := filepath.Dir(filepath.FromSlash(rel))
	if parent == "." {
		return "."
	}
	return filepath.ToSlash(parent)
}

func joinRelPath(base, name string) string {
	if base == "." || base == "" {
		return filepath.ToSlash(name)
	}
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(base), name))
}

func (m *browseModel) scopeRefForPathRel(rel string) string {
	if rel == "." || rel == "" {
		return "path:./"
	}
	return "path:./" + filepath.ToSlash(rel)
}

func browseItemForProviderChild(child refpkg.ScopeChild) browseItem {
	ref := child.Ref.String()
	name := filepath.Base(filepath.FromSlash(strings.Trim(child.Ref.Path, "/")))
	switch child.Kind {
	case refpkg.ScopeChildFile:
		return browseItem{
			kind:      browseItemFile,
			title:     name,
			desc:      "module",
			targetRef: ref,
		}
	default:
		return browseItem{
			kind:      browseItemDir,
			title:     name + "/",
			desc:      "subpackage",
			targetRef: ref,
		}
	}
}

func browseScopeFromReference(ref ingest.Reference) (rootDir string, currentRel string, err error) {
	if ref.Path == "" || ref.Path == "." || ref.Path == "./" {
		return ".", ".", nil
	}

	pathValue := ref.Path
	if !filepath.IsAbs(pathValue) {
		pathValue = strings.TrimPrefix(pathValue, "./")
		if pathValue == "" {
			pathValue = "."
		}
		pathValue, err = filepath.Abs(pathValue)
		if err != nil {
			return "", "", err
		}
	}

	st, err := os.Stat(pathValue)
	if err != nil {
		return "", "", err
	}

	if st.IsDir() {
		return pathValue, ".", nil
	}
	return filepath.Dir(pathValue), filepath.Base(pathValue), nil
}

func docToMarkdown(doc *ingest.DocResult, fenceLanguage string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n", doc.Name)
	if doc.Signature != "" {
		if fenceLanguage != "" {
			out.WriteString("```" + fenceLanguage + "\n")
		} else {
			out.WriteString("```\n")
		}
		out.WriteString(doc.Signature)
		out.WriteString("\n```\n")
	}
	if doc.DocString != "" {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(doc.DocString)
		out.WriteByte('\n')
	}
	return strings.TrimRight(out.String(), "\n")
}

func markdownFenceLanguageForRef(ref string) string {
	parsed := ingest.ParseReference(ref)
	if parsed.Provider == "path" {
		if lang, ok := ingest.LanguageForFile(parsed.Path); ok {
			return lang
		}
		return ""
	}

	switch parsed.Provider {
	case "node":
		return "javascript"
	case "go", "python", "javascript", "java":
		return parsed.Provider
	default:
		return ""
	}
}
