package skills

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type skMode int

const (
	skModeList skMode = iota
	skModeInstall
	skModePick
	skModeConfirm
	skModeDoc
	skModeNew         // new skill name input
	skModeProfiles    // profile list
	skModeProfileName // profile name input
	skModeBackup      // a skill's backup list
	skModeRegistry    // registry search input
	skModeRegistryPick
)

// Tab is the Skills tab: the skill × agent matrix, toggled by symlink.
type Tab struct {
	svc     *Service
	agents  []agent.Agent // all detected
	targets []agent.Agent // installed with a skills dir (matrix columns)
	skills  []Skill

	list       list.Model
	input      textinput.Model
	picker     picker
	confirm    components.Confirm
	vp         viewport.Model
	detailVP   viewport.Model // scrollable detail content
	col        int            // agent under the cursor (index in targets)
	detailName string         // skill in the detail, to reset the scroll on change
	docName    string
	docSource  string // raw markdown, re-rendered after resize/theme changes
	docPath    string // folder of the skill open in the reader

	mode           skMode
	ckind          confirmKind
	pendingRemove  Skill
	pendingUpdate  Skill
	profileNames   []string
	profileCursor  int
	pendingProfile string
	pdiff          profileDiff
	updateChecks   []UpdateCheck
	updateStatus   map[string]UpdateStatus
	backupPicker   backupPickerState
	pendingRestore Backup
	regPicker      registryPickerState
	toast          string
	toastErr       bool
	toastSeq       int           // guards the toast against stale expiry timers
	spin           spinner.Model // network operation animation
	inFlight       bool          // network operation in flight
	width, height  int
}

// skToastExpire clears the toast if it is still number seq.
type skToastExpire struct{ seq int }

func expireToastCmd(seq int) tea.Cmd {
	return tea.Tick(kit.ToastTTL, func(time.Time) tea.Msg { return skToastExpire{seq} })
}

func newTab(svc *Service) Tab {
	l := list.New(nil, kit.TableDelegate{}, 0, 0)
	kit.StyleList(&l)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)  // "N items" is in the Panel title
	l.SetShowPagination(false) // no raw dots
	l.DisableQuitKeybindings()
	in := components.NewInput()
	in.Placeholder = "GitHub URL, user/repo, folder or .zip file"
	in.CharLimit = 1024
	in.SetWidth(60)
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(theme.Primary)))
	return Tab{svc: svc, list: l, input: in, vp: viewport.New(), detailVP: viewport.New(), spin: sp}
}

// beginSpin starts the spinner with a progress label and returns the first
// tick; with an operation already in flight it only swaps the label.
func (m *Tab) beginSpin(label string) tea.Cmd {
	m.toast, m.toastErr = label, false
	if m.inFlight {
		return nil
	}
	m.inFlight = true
	return m.spin.Tick
}

func (m Tab) Init() tea.Cmd { return nil }

func (m Tab) Capturing() bool {
	return m.mode != skModeList || m.list.SettingFilter()
}

// Update wraps update() to schedule toast auto-dismiss: a new toast text bumps
// seq and schedules its expiry.
func (m Tab) step(msg tea.Msg) (Tab, tea.Cmd) {
	prev := m.toast
	var cmd tea.Cmd
	m, cmd = m.update(msg)
	if m.toast != "" && m.toast != prev {
		m.toastSeq++
		cmd = tea.Batch(cmd, expireToastCmd(m.toastSeq))
	}
	return m, cmd
}

func (m Tab) update(msg tea.Msg) (Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case skToastExpire:
		if msg.seq == m.toastSeq && !m.inFlight {
			m.toast = ""
		}
		return m, nil

	case spinner.TickMsg:
		if !m.inFlight {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case events.AgentsDetected:
		m.agents = msg.Agents
		m.targets = nil
		for _, ag := range msg.Agents {
			if ag.Installed && ag.SupportsSkills() {
				m.targets = append(m.targets, ag)
			}
		}
		m.col = max(0, min(m.col, len(m.targets)-1))
		return m, m.scanCmd()

	case scannedMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.skills = msg.skills
		return m, tea.Batch(m.rebuildListItems(), m.announceCmd())

	case listBackupsMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		if len(msg.backups) == 0 {
			m.setToast(fmt.Sprintf("no backups found for %q", msg.skillDir), false)
			return m, nil
		}
		m.backupPicker = newBackupPicker(msg.backups, msg.skillDir)
		m.mode = skModeBackup
		return m, nil

	case restoreBackupMsg:
		m.mode = skModeList
		if msg.err != nil {
			m.setToast(fmt.Sprintf("restoring %s failed: %s", msg.skillDir, msg.err), true)
		} else {
			m.setToast(fmt.Sprintf("%s restored", msg.skillDir), false)
		}
		return m, m.scanCmd()

	case checkUpdatesMsg:
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		if len(msg.checks) == 0 {
			m.setToast("no git skills to check", false)
			return m, nil
		}
		m.updateChecks = msg.checks
		m.updateStatus = make(map[string]UpdateStatus, len(msg.checks))
		var available, localEdited int
		for _, ch := range msg.checks {
			m.updateStatus[ch.Skill.Dir] = ch.Status
			switch ch.Status {
			case UpdateStatusAvailable:
				available++
			case UpdateStatusLocallyEdited:
				localEdited++
			}
		}
		if available == 0 {
			count := len(msg.checks)
			m.setToast(fmt.Sprintf("%d skill(s) up to date", count), false)
			m.updateChecks = nil
			return m, m.rebuildListItems()
		}
		cmsg := fmt.Sprintf("Update %d skill(s) from GitHub?", available)
		if localEdited > 0 {
			cmsg = fmt.Sprintf("Update %d skill(s) from GitHub? (%d edited locally will be skipped)", available, localEdited)
		}
		m.ckind = confirmKindUpdateAll
		m.confirm = components.NewConfirm(cmsg)
		m.mode = skModeConfirm
		return m, m.rebuildListItems()

	case updateAllMsg:
		m.inFlight = false
		m.mode = skModeList
		m.updateChecks = nil
		if len(msg.errs) > 0 {
			parts := make([]string, len(msg.errs))
			for i, e := range msg.errs {
				parts[i] = e.Error()
			}
			m.setToast(fmt.Sprintf(
				"%d updated, %d failed: %s",
				msg.updated, len(msg.errs), strings.Join(parts, "; "),
			), true)
		} else if len(msg.skipped) > 0 {
			m.setToast(fmt.Sprintf(
				"%d updated, skipped (edited locally): %s",
				msg.updated, strings.Join(msg.skipped, ", "),
			), false)
		} else {
			m.setToast(fmt.Sprintf("%d skill(s) updated", msg.updated), false)
		}
		return m, m.scanCmd()

	case adoptAllDoneMsg:
		m.inFlight = false
		m.mode = skModeList
		if len(msg.errs) > 0 {
			parts := make([]string, len(msg.errs))
			for i, e := range msg.errs {
				parts[i] = e.Error()
			}
			m.setToast(fmt.Sprintf(
				"%d adopted, %d failed: %s",
				len(msg.adopted), len(msg.errs), strings.Join(parts, "; "),
			), true)
		} else if len(msg.adopted) == 0 {
			m.setToast("no local skills to adopt", false)
		} else {
			m.setToast(fmt.Sprintf("%d skill(s) adopted: %s", len(msg.adopted), strings.Join(msg.adopted, ", ")), false)
		}
		return m, m.scanCmd()

	case skillOpMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast(msg.verb, false)
		}
		return m, m.scanCmd()

	case discoverMsg:
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		if len(msg.found) == 1 && len(msg.origin.Notes) == 0 {
			// one-skill repo: install directly; only multi-skill sources or
			// marketplace warnings need the picker.
			m.mode = skModeList
			svc, origin, cleanup, found := m.svc, msg.origin, msg.cleanup, msg.found
			spin := m.beginSpin(fmt.Sprintf("installing %s…", found[0].Name))
			return m, tea.Batch(spin, func() tea.Msg {
				names, err := svc.Install(found, origin)
				if cleanup != "" {
					os.RemoveAll(cleanup)
				}
				return installDoneMsg{names: names, err: err}
			})
		}
		m.picker = newPicker(msg.found, msg.origin, msg.cleanup)
		m.mode = skModePick
		return m, nil

	case installDoneMsg:
		m.inFlight = false
		m.mode = skModeList
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else if len(msg.names) > 0 {
			m.setToast("installed: "+strings.Join(msg.names, ", "), false)
		} else {
			m.setToast("nothing installed", true)
		}
		return m, m.scanCmd()

	case registrySearchMsg:
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		m.regPicker = newRegistryPicker(msg.results)
		m.mode = skModeRegistryPick
		return m, nil

	case docMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		stayPut := m.mode == skModeDoc && m.docName == msg.name // reload after editing
		m.docName, m.docPath = msg.name, msg.path
		m.docSource = msg.content
		m.vp.SetContent(kit.RenderMarkdown(msg.content, m.width-2))
		if !stayPut {
			m.vp.GotoTop()
		}
		m.mode = skModeDoc
		return m, nil

	case editDoneMsg:
		if msg.err != nil {
			m.setToast("editor exited with an error: "+msg.err.Error(), true)
		} else {
			m.setToast(fmt.Sprintf("SKILL.md of %s saved", msg.name), false)
		}
		cmds := []tea.Cmd{m.scanCmd()}
		if m.mode == skModeDoc {
			cmds = append(cmds, loadDocCmd(m.docName, m.docPath))
		}
		return m, tea.Batch(cmds...)

	case createdMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil // stay in the input to fix the name
		}
		m.mode = skModeList
		m.setToast(fmt.Sprintf("skill %s created — opening editor", msg.name), false)
		return m, editCmd(msg.name, msg.path)

	case updateDoneMsg:
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast(fmt.Sprintf("%s updated (backup in %s)", msg.name, core.Tilde(m.svc.Paths().BackupsDir(), m.svc.Paths().Home)), false)
		}
		return m, m.scanCmd()

	case profilesLoadMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.profileNames = msg.names
		m.profileCursor = 0
		m.mode = skModeProfiles
		return m, nil

	case profileDiffMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.pendingProfile = msg.name
		m.pdiff = profileDiff{name: msg.name, changes: msg.changes}
		var lines []string
		lines = append(lines, fmt.Sprintf("Apply profile %q?", msg.name))
		if len(msg.changes) == 0 {
			lines = append(lines, kit.StHint.Render("(no changes — already matches the profile)"))
		}
		for _, c := range msg.changes {
			if len(c.Add) > 0 {
				lines = append(lines, kit.StAdded.Render("+ "+c.Skill+": ")+m.agentLabels(c.Add))
			}
			if len(c.Remove) > 0 {
				lines = append(lines, kit.StRemoved.Render("− "+c.Skill+": ")+m.agentLabels(c.Remove))
			}
		}
		m.confirm = components.NewConfirm(strings.Join(lines, "\n"))
		m.ckind = confirmKindApplyProfile
		m.mode = skModeConfirm
		return m, nil

	case profileSaveMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeProfileName
			return m, m.input.Focus()
		}
		m.setToast(fmt.Sprintf("profile %q saved", msg.name), false)
		return m, m.loadProfilesCmd()

	case profileApplyDoneMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast(fmt.Sprintf("profile %q applied", msg.name), false)
		}
		m.mode = skModeList
		return m, m.scanCmd()

	case tea.MouseWheelMsg:
		switch m.mode {
		case skModeConfirm:
			m.confirm, _ = m.confirm.Update(msg, m.width, m.height)
		case skModeDoc:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		case skModeList:
			sp := m.split()
			if sp.Side && msg.X >= sp.ListW || !sp.Side && msg.Y >= sp.ListH {
				var cmd tea.Cmd
				m.detailVP, cmd = m.detailVP.Update(msg) // wheel over the detail scrolls it
				return m, cmd
			}
			if msg.Button == tea.MouseWheelUp {
				m.list.CursorUp()
			} else if msg.Button == tea.MouseWheelDown {
				m.list.CursorDown()
			}
			m.refreshDetail()
		case skModePick:
			p := &m.picker
			if msg.Button == tea.MouseWheelUp && p.cursor > 0 {
				p.cursor--
			} else if msg.Button == tea.MouseWheelDown && p.cursor < len(p.items)-1 {
				p.cursor++
			}
		case skModeRegistryPick:
			p := &m.regPicker
			if msg.Button == tea.MouseWheelUp && p.cursor > 0 {
				p.cursor--
			} else if msg.Button == tea.MouseWheelDown && p.cursor < len(p.items)-1 {
				p.cursor++
			}
		case skModeProfiles:
			if msg.Button == tea.MouseWheelUp && m.profileCursor > 0 {
				m.profileCursor--
			} else if msg.Button == tea.MouseWheelDown && m.profileCursor < len(m.profileNames)-1 {
				m.profileCursor++
			}
		}
		return m, nil

	case tea.MouseClickMsg:
		return m.click(msg)

	case tea.PasteMsg:
		var cmd tea.Cmd
		switch {
		case m.mode == skModeInstall, m.mode == skModeNew, m.mode == skModeProfileName, m.mode == skModeRegistry:
			m.input, cmd = m.input.Update(msg) // textinput handles paste natively
		case m.mode == skModeList && m.list.SettingFilter():
			m.list, cmd = kit.FeedTextToList(m.list, msg.Content)
		}
		return m, cmd

	case tea.KeyPressMsg:
		switch m.mode {
		case skModeInstall:
			return m.updateInstall(msg)
		case skModeNew:
			return m.updateNew(msg)
		case skModePick:
			return m.updatePick(msg)
		case skModeRegistry:
			return m.updateRegistrySearch(msg)
		case skModeRegistryPick:
			return m.updateRegistryPick(msg)
		case skModeConfirm:
			return m.updateConfirm(msg)
		case skModeDoc:
			return m.updateDoc(msg)
		case skModeProfiles:
			return m.updateProfiles(msg)
		case skModeProfileName:
			return m.updateProfileName(msg)
		case skModeBackup:
			return m.updateBackupPicker(msg)
		default:
			return m.updateList(msg)
		}
	}
	// bubbles' internal messages (e.g. list.FilterMatchesMsg, the async filter
	// result) must reach the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// bodyHeight is the height left for the body (minus hints and toast).
func (m Tab) bodyHeight() int {
	h := m.height - 2
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Tab) layout() {
	if m.width == 0 {
		return
	}
	bodyH := m.bodyHeight()
	// The list only keeps filter and cursor (the matrix is drawn separately); one
	// line per skill so PgUp/PgDn move a screen.
	sp := m.split()
	m.list.SetSize(sp.ListW, max(1, sp.ListH-2))
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(bodyH)
	if m.mode == skModeDoc {
		m.vp.SetContent(kit.RenderMarkdown(m.docSource, m.width-2))
	}
	m.refreshDetail()
}

func (m *Tab) setToast(s string, isErr bool) {
	m.toast, m.toastErr = s, isErr
}

// ClearToast drops the toast (on tab switch).
func (m *Tab) ClearToast() { m.toast = "" }

// Count is the number of library skills (tab counter).
func (m Tab) Count() int { return len(m.skills) }

func (m *Tab) ID() string { return "skills" }

func (m *Tab) Title() string { return "Skills" }

// Update applies the message and keeps the new state (module.Module pointer
// semantics). events.Reload is the same as the r key.
func (m *Tab) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(events.Reload); ok {
		msg = tea.KeyPressMsg{Code: 'r', Text: "r"}
	}
	nm, cmd := m.step(msg)
	*m = nm
	return cmd
}
