package skills

import (
	"github.com/rogeriojunior31/lazyagents/internal/skill"
)

type profileDiff struct {
	name    string
	changes []skill.ProfileChange
}

type skillOpMsg struct {
	verb string
	err  error
}

type discoverMsg struct {
	found   []skill.Found
	origin  skill.Origin
	cleanup string
	err     error
}

type installDoneMsg struct {
	names []string
	err   error
}

type registrySearchMsg struct {
	results []skill.RegistryResult
	err     error
}

type docMsg struct {
	name    string
	path    string // pasta da skill
	content string
	err     error
}

type editDoneMsg struct {
	name string
	err  error
}

type createdMsg struct {
	name string
	path string
	err  error
}

type updateDoneMsg struct {
	name string
	err  error
}

type profilesLoadMsg struct {
	names []string
	err   error
}

type profileDiffMsg struct {
	name    string
	changes []skill.ProfileChange
	err     error
}

type profileSaveMsg struct {
	name string
	err  error
}

type listBackupsMsg struct {
	backups  []skill.Backup
	skillDir string
	err      error
}

type restoreBackupMsg struct {
	skillDir string
	err      error
}

type checkUpdatesMsg struct {
	checks []skill.UpdateCheck
	err    error
}

type updateAllMsg struct {
	updated int
	skipped []string
	errs    []error
}

type profileApplyDoneMsg struct {
	name string
	err  error
}

type adoptAllDoneMsg struct {
	adopted []string
	errs    []error
}
