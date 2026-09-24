package skills

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestBackupPickerSmallTerminal(t *testing.T) {
	backups := make([]Backup, 20)
	for i := range backups {
		backups[i] = Backup{Path: fmt.Sprintf("/backups/skill-%02d.tar.gz", i), Time: time.Date(2026, 9, i+1, 12, 0, 0, 0, time.UTC)}
	}
	picker := newBackupPicker(backups, "skill")
	for _, width := range []int{30, 36, 76} {
		for i := range backups {
			picker.cursor = i
			view := picker.view(width, 11)
			plain := ansi.Strip(view)
			if lipgloss.Height(view) > 11 || lipgloss.Width(view) > width || !strings.Contains(plain, backups[i].Time.Format("02/01/2006")) || !strings.Contains(plain, "esc") {
				t.Fatalf("backup %d em %d colunas inacessível:\n%s", i, width, plain)
			}
		}
	}
}
