// Package filter applies query and sort pipelines to task collections.
package filter

import (
	"fmt"
	"time"

	"github.com/zachthieme/pike/internal/config"
	"github.com/zachthieme/pike/internal/model"
	"github.com/zachthieme/pike/internal/query"
	tasksort "github.com/zachthieme/pike/internal/sort"
)

// ViewResult holds the filtered and sorted tasks for a single view.
type ViewResult struct {
	Title string
	Color string
	Tasks []model.Task
}

// Apply filters and sorts tasks for a single query+sort combo.
func Apply(tasks []model.Task, queryStr string, sortOrder string, now time.Time) ([]model.Task, error) {
	node, err := query.Parse(queryStr)
	if err != nil {
		return nil, err
	}

	// A nil node (empty query) matches all tasks.
	var matched []model.Task
	for i := range tasks {
		if node == nil || query.Eval(node, &tasks[i], now) {
			matched = append(matched, tasks[i])
		}
	}

	// Ensure we return a non-nil slice even if no tasks matched.
	if matched == nil {
		matched = []model.Task{}
	}

	// Sort results if a sort order is specified.
	if sortOrder != "" {
		if err := tasksort.Sort(matched, sortOrder); err != nil {
			return nil, err
		}
	}

	matched = tasksort.StablePartitionPinned(matched)

	return matched, nil
}

// ApplyViews runs Apply for each ViewConfig and returns results.
// Views with Hidden: true are skipped and excluded from the results.
// Children of matched parents are included even if they don't independently
// match the query, so that subtask collapse/expand works in the TUI.
func ApplyViews(tasks []model.Task, views []config.ViewConfig, now time.Time) ([]ViewResult, error) {
	results := make([]ViewResult, 0, len(views))

	for _, view := range views {
		if view.Hidden {
			continue
		}
		filtered, err := Apply(tasks, view.Query, view.Sort, now)
		if err != nil {
			return nil, err
		}

		filtered = includeChildren(filtered, tasks)

		results = append(results, ViewResult{
			Title: view.Title,
			Color: view.Color,
			Tasks: filtered,
		})
	}

	return results, nil
}

// includeChildren adds children of matched parents that aren't already in the
// result set. This ensures subtask collapse/expand works even when children
// don't independently match the view query. Children are inserted right after
// their parent to maintain visual grouping.
func includeChildren(matched []model.Task, allTasks []model.Task) []model.Task {
	if len(matched) == 0 {
		return matched
	}

	// Build set of matched task keys
	present := make(map[string]bool, len(matched))
	for _, t := range matched {
		present[taskKey(t)] = true
	}

	// Check if any parents exist — fast path
	hasParents := false
	for _, t := range matched {
		if len(t.Children) > 0 {
			hasParents = true
			break
		}
	}
	if !hasParents {
		return matched
	}

	// Find parents in matched set and collect their missing children
	var result []model.Task
	for _, t := range matched {
		result = append(result, t)
		if len(t.Children) == 0 {
			continue
		}
		for _, child := range t.Children {
			if !present[taskKey(*child)] {
				result = append(result, *child)
				present[taskKey(*child)] = true
			}
		}
	}
	return result
}

func taskKey(t model.Task) string {
	return t.File + ":" + fmt.Sprintf("%d", t.Line)
}
