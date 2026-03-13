package sort

import (
	"cmp"
	"fmt"
	"slices"
	"pike/internal/model"
	"time"
)

// Sort sorts tasks in place according to the given sort order.
// Returns an error for unknown sort orders.
func Sort(tasks []model.Task, order string) error {
	var comparator func(a, b model.Task) int

	switch order {
	case "due_asc":
		comparator = func(a, b model.Task) int {
			return compareDatesNilLast(a.Due, b.Due, false)
		}
	case "due_desc":
		comparator = func(a, b model.Task) int {
			return compareDatesNilLast(a.Due, b.Due, true)
		}
	case "completed_asc":
		comparator = func(a, b model.Task) int {
			return compareDatesNilLast(a.Completed, b.Completed, false)
		}
	case "completed_desc":
		comparator = func(a, b model.Task) int {
			return compareDatesNilLast(a.Completed, b.Completed, true)
		}
	case "file":
		comparator = func(a, b model.Task) int {
			if c := cmp.Compare(a.File, b.File); c != 0 {
				return c
			}
			return cmp.Compare(a.Line, b.Line)
		}
	case "alpha":
		comparator = func(a, b model.Task) int {
			return cmp.Compare(a.Text, b.Text)
		}
	default:
		return fmt.Errorf("unknown sort order: %q", order)
	}

	slices.SortStableFunc(tasks, comparator)
	return nil
}

// compareDatesNilLast compares two date pointers, placing nil values last.
// When desc is true, non-nil dates are sorted in descending order.
func compareDatesNilLast(a, b *time.Time, desc bool) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1 // a (nil) goes after b
	}
	if b == nil {
		return -1 // a goes before b (nil)
	}

	if desc {
		return b.Compare(*a)
	}
	return a.Compare(*b)
}
