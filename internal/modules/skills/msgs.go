package skills

type profileDiff struct {
	name    string
	changes []ProfileChange
}

type skillOpMsg struct {
	verb string
	err  error
}

type discoverMsg struct {
	found   []Found
	origin  Origin
	cleanup string
	err     error
}

type installDoneMsg struct {
	names []string
	err   error
}

type registrySearchMsg struct {
	results []RegistryResult
	err     error
}

type docMsg struct {
	name    string
	path    string // skill folder
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
	changes []ProfileChange
	err     error
}

type profileSaveMsg struct {
	name string
	err  error
}

type listBackupsMsg struct {
	backups  []Backup
	skillDir string
	err      error
}

type restoreBackupMsg struct {
	skillDir string
	err      error
}

type checkUpdatesMsg struct {
	checks []UpdateCheck
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

// scannedMsg is the library scan result. Internal: other tabs see the
// events.SkillsScanned aggregate.
type scannedMsg struct {
	skills []Skill
	err    error
}
