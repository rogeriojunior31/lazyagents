package skills

import (
	"fmt"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Keep the M25 release-gate fixture constant so timings remain comparable.
func BenchmarkInventoryMaintenance(b *testing.B) {
	m := newTab(New(core.PathsIn(b.TempDir())))
	m.width, m.height = 120, 30
	for i := range 1000 {
		m.skills = append(m.skills, Skill{Dir: fmt.Sprintf("skill-%04d", i), Name: fmt.Sprintf("Skill %04d", i), Description: "Review database migrations and tests", InLibrary: true, Valid: true})
	}
	m.rebuildListItems()
	b.Run("filter1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				m.filters.location = "library"
			} else {
				m.filters.location = ""
			}
			m.applyFilters("", 0)
		}
	})
	b.Run("scroll1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m.list.Select(i % 1000)
			m.tableWindow(120, 30)
		}
	})
}
