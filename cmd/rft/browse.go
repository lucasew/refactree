package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
	"github.com/spf13/cobra"
)

func newBrowseCmd(root *rootOptions) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "browse [reference]",
		Short: "Interactive symbol browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refInput := "path:./"
			if len(args) == 1 {
				refInput = args[0]
			}

			ref := coerceLocalPathRef(ingest.ParseReference(refInput))
			if ref.Provider == "" {
				ref.Provider = "path"
				if ref.Path == "" {
					ref.Path = "./"
				}
			}
			ref.Symbol = ""

			model, err := newBrowseModelFromReference(ref, all)
			if err != nil {
				return err
			}

			final, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
			if err != nil {
				return err
			}

			out, ok := final.(*browseModel)
			if !ok {
				return nil
			}
			return out.err
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "show hidden packages and symbols")
	return cmd
}

type browseFocus int

const (
	browseFocusList browseFocus = iota
	browseFocusPreview
)

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

func newBrowseKeys() browseKeys {
	return browseKeys{
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Parent: key.NewBinding(
			key.WithKeys("h", "backspace"),
			key.WithHelp("h", "go parent"),
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
	currentRel    string
	mode          string
	providerRef   ingest.Reference
	providerDir   string
	includeHidden bool
	focus         browseFocus
	showFullHelp  bool
	keys          browseKeys

	list    list.Model
	preview viewport.Model
	help    help.Model

	docCache    map[string]string
	loadingDocs map[string]bool
	err         error

	width        int
	height       int
	listWidth    int
	previewWidth int
	bodyHeight   int
}

func newBrowseModelFromReference(ref ingest.Reference, includeHidden bool) (*browseModel, error) {
	if ref.Provider == "path" {
		rootDir, currentRel, err := browseScopeFromReference(ref)
		if err != nil {
			return nil, err
		}
		return newBrowseModel(rootDir, currentRel, includeHidden)
	}

	scope, ok, err := refpkg.ResolveScopeTarget(ref)
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
		currentRel:    currentRel,
		mode:          "path",
		includeHidden: includeHidden,
		focus:         browseFocusList,
		keys:          newBrowseKeys(),
		docCache:      map[string]string{},
		loadingDocs:   map[string]bool{},
	}
	model.help.ShowAll = false

	model.list = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	model.list.Title = "rft browse"
	model.list.SetShowHelp(false)
	model.list.SetShowStatusBar(false)
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
	return m.ensureSelectedDocLoadedCmd()
}

func (m *browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.updatePreviewForSelection()
		return m, m.ensureSelectedDocLoadedCmd()
	case docLoadedMsg:
		delete(m.loadingDocs, msg.ref)
		m.docCache[msg.ref] = msg.markdown
		if m.selectedSymbolRef() == msg.ref {
			m.updatePreviewForSelection()
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleHelp):
			m.showFullHelp = !m.showFullHelp
			m.help.ShowAll = m.showFullHelp
			m.resize()
			m.updatePreviewForSelection()
			return m, m.ensureSelectedDocLoadedCmd()
		case key.Matches(msg, m.keys.ToggleAll):
			m.includeHidden = !m.includeHidden
			if err := m.reload(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.ensureSelectedDocLoadedCmd()
		case key.Matches(msg, m.keys.Refresh):
			if err := m.reload(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.ensureSelectedDocLoadedCmd()
		case key.Matches(msg, m.keys.Parent):
			if err := m.goParent(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.ensureSelectedDocLoadedCmd()
		case key.Matches(msg, m.keys.ToggleFocus):
			if m.focus == browseFocusList {
				m.focus = browseFocusPreview
			} else {
				m.focus = browseFocusList
			}
			m.updatePreviewForSelection()
			return m, m.ensureSelectedDocLoadedCmd()
		case key.Matches(msg, m.keys.Open):
			if err := m.activateSelection(); err != nil {
				m.err = err
				return m, tea.Quit
			}
			return m, m.ensureSelectedDocLoadedCmd()
		}

		if m.focus == browseFocusPreview {
			switch {
			case key.Matches(msg, m.keys.PreviewUp):
				m.preview.LineUp(1)
				return m, nil
			case key.Matches(msg, m.keys.PreviewDown):
				m.preview.LineDown(1)
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
			return m, tea.Batch(cmd, m.ensureSelectedDocLoadedCmd())
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

	m.list.Title = fmt.Sprintf("%s  (%s)", current, focusLabel)

	leftStyle := lipgloss.NewStyle().Width(m.listWidth).Height(m.bodyHeight)
	rightStyle := lipgloss.NewStyle().Width(m.previewWidth).Height(m.bodyHeight).PaddingLeft(1)
	if m.focus == browseFocusList {
		leftStyle = leftStyle.Border(lipgloss.NormalBorder(), false, true, false, false)
	}
	if m.focus == browseFocusPreview {
		rightStyle = rightStyle.Border(lipgloss.NormalBorder(), false, false, false, true)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftStyle.Render(m.list.View()), rightStyle.Render(m.preview.View()))
	status := fmt.Sprintf("root:%s  scope:%s  %s", m.scopeRoot(), current, mode)

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
		}

		sort.Slice(dirs, func(i, j int) bool { return dirs[i].title < dirs[j].title })
		for _, it := range dirs {
			items = append(items, it)
		}
	}

	symbols, err := m.symbolItems()
	if err != nil {
		return nil, err
	}
	for _, sym := range symbols {
		items = append(items, sym)
	}

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
	scope, ok, err := refpkg.ResolveScopeTarget(m.providerRef)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("provider scope navigation not supported for %q", m.providerRef.Provider)
	}
	m.providerDir = scope.Dir

	items := make([]list.Item, 0, 32)
	parent := parentProviderPath(m.providerRef.Path)
	if parent != m.providerRef.Path {
		items = append(items, browseItem{
			kind:      browseItemParent,
			title:     "..",
			desc:      "parent package",
			targetRef: ingest.Reference{Provider: m.providerRef.Provider, Path: parent}.String(),
		})
	}

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
		childDir := filepath.Join(m.providerDir, name)
		if m.providerRef.Provider == "go" && !dirHasGoSources(childDir) {
			continue
		}
		childPath := joinProviderPath(m.providerRef.Path, name)
		packages = append(packages, browseItem{
			kind:      browseItemDir,
			title:     name + "/",
			desc:      "subpackage",
			targetRef: ingest.Reference{Provider: m.providerRef.Provider, Path: childPath}.String(),
		})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].title < packages[j].title })
	for _, pkg := range packages {
		items = append(items, pkg)
	}

	symbols, err := m.symbolItems()
	if err != nil {
		return nil, err
	}
	for _, sym := range symbols {
		items = append(items, sym)
	}

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
	ref := m.currentScopeRef()
	options := ingest.ListOptions{IncludeHidden: m.includeHidden}
	items := make([]browseItem, 0, 64)

	dir := m.rootDir
	if m.mode == "provider" {
		dir = "."
	}
	err := ingest.WalkSymbols(dir, ref, options, func(sym ingest.SymbolInfo) bool {
		path := strings.TrimPrefix(sym.Reference.Path, "./")
		if path == "" {
			path = "."
		}
		items = append(items, browseItem{
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

	sort.Slice(items, func(i, j int) bool {
		if items[i].title != items[j].title {
			return items[i].title < items[j].title
		}
		if items[i].desc != items[j].desc {
			return items[i].desc < items[j].desc
		}
		return items[i].symbolRef < items[j].symbolRef
	})

	out := make([]list.Item, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out, nil
}

func (m *browseModel) updatePreviewForSelection() {
	item, ok := m.list.SelectedItem().(browseItem)
	if !ok {
		m.preview.SetContent("No selection")
		return
	}

	lines := []string{
		fmt.Sprintf("scope: %s", m.currentScopeRef()),
		fmt.Sprintf("root:  %s", m.scopeRoot()),
		"",
		fmt.Sprintf("entry: %s", item.title),
		fmt.Sprintf("kind:  %s", item.kindName()),
	}

	switch item.kind {
	case browseItemParent, browseItemDir, browseItemFile:
		target := item.targetRel
		if item.targetRef != "" {
			target = item.targetRef
		}
		lines = append(lines, fmt.Sprintf("target: %s", target))
		if item.kind == browseItemFile {
			lines = append(lines, "", "Press Enter to focus this file and list only its symbols.")
		} else {
			lines = append(lines, "", "Press Enter to enter.")
		}
	case browseItemSymbol:
		lines = append(lines, fmt.Sprintf("ref:   %s", item.symbolRef))
		if doc, ok := m.docCache[item.symbolRef]; ok {
			lines = append(lines, "", m.renderMarkdown(doc))
		} else if m.loadingDocs[item.symbolRef] {
			lines = append(lines, "", "Loading documentation...")
		} else {
			lines = append(lines, "", "Loading documentation...")
		}
	}

	m.preview.SetContent(strings.Join(lines, "\n"))
	m.preview.GotoTop()
}

func (m *browseModel) activateSelection() error {
	item, ok := m.list.SelectedItem().(browseItem)
	if !ok {
		return nil
	}

	switch item.kind {
	case browseItemParent, browseItemDir, browseItemFile:
		if m.mode == "provider" {
			if item.targetRef == "" {
				return nil
			}
			ref := ingest.ParseReference(item.targetRef)
			ref.Symbol = ""
			scope, ok, err := refpkg.ResolveScopeTarget(ref)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("provider scope navigation not supported for %q", ref.Provider)
			}
			m.providerRef = ref
			m.providerDir = scope.Dir
			m.list.Select(0)
			return m.reload()
		}
		if err := m.setCurrentRel(item.targetRel); err != nil {
			return err
		}
		m.list.Select(0)
		return m.reload()
	case browseItemSymbol:
		m.updatePreviewForSelection()
	}
	return nil
}

func (m *browseModel) goParent() error {
	if m.mode == "provider" {
		parent := parentProviderPath(m.providerRef.Path)
		if parent == m.providerRef.Path {
			return nil
		}
		ref := m.providerRef
		ref.Path = parent
		scope, ok, err := refpkg.ResolveScopeTarget(ref)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("provider scope navigation not supported for %q", ref.Provider)
		}
		m.providerRef = ref
		m.providerDir = scope.Dir
		m.list.Select(0)
		return m.reload()
	}

	if m.currentRel == "." {
		return nil
	}
	if err := m.setCurrentRel(parentRel(m.currentRel)); err != nil {
		return err
	}
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

func (m *browseModel) docLookupDir() string {
	if m.mode == "provider" {
		return "."
	}
	return m.rootDir
}

func (m *browseModel) selectedSymbolRef() string {
	item, ok := m.list.SelectedItem().(browseItem)
	if !ok || item.kind != browseItemSymbol || item.symbolRef == "" {
		return ""
	}
	return item.symbolRef
}

func (m *browseModel) ensureSelectedDocLoadedCmd() tea.Cmd {
	ref := m.selectedSymbolRef()
	if ref == "" {
		return nil
	}
	if _, ok := m.docCache[ref]; ok {
		return nil
	}
	if m.loadingDocs[ref] {
		return nil
	}

	dir := m.docLookupDir()
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
			markdown: docToMarkdown(doc),
		}
	}
}

func (m *browseModel) renderMarkdown(markdown string) string {
	if markdown == "" {
		return ""
	}

	wrap := m.previewWidth
	if wrap <= 0 {
		wrap = 80
	}
	// Keep a small margin so wrapped markdown doesn't touch viewport edges.
	if wrap > 4 {
		wrap -= 2
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrap),
	)
	if err != nil {
		return markdown
	}
	out, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return strings.TrimRight(out, "\n")
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

func parentProviderPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
	if parent == "." {
		return ""
	}
	return parent
}

func joinProviderPath(base, name string) string {
	base = strings.Trim(base, "/")
	name = strings.Trim(name, "/")
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + "/" + name
}

func dirHasGoSources(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			return true
		}
	}
	return false
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

func docToMarkdown(doc *ingest.DocResult) string {
	lines := []string{fmt.Sprintf("# %s", doc.Name)}
	if doc.Signature != "" {
		lines = append(lines, "```")
		lines = append(lines, doc.Signature)
		lines = append(lines, "```")
	}
	if doc.DocString != "" {
		lines = append(lines, "", doc.DocString)
	}
	return strings.Join(lines, "\n")
}
