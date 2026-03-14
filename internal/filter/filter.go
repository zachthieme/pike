package filter

import (
	"pike/internal/config"
	"pike/internal/model"
	"pike/internal/query"
	tasksort "pike/internal/sort"
	"time"
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

	// Filter tasks that match the query.
	var matched []model.Task
	for i := range tasks {
		if query.Eval(node, &tasks[i], now) {
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
func ApplyViews(tasks []model.Task, views []config.ViewConfig, now time.Time) ([]ViewResult, error) {
	results := make([]ViewResult, 0, len(views))

	for _, view := range views {
		filtered, err := Apply(tasks, view.Query, view.Sort, now)
		if err != nil {
			return nil, err
		}

		results = append(results, ViewResult{
			Title: view.Title,
			Color: view.Color,
			Tasks: filtered,
		})
	}

	return results, nil
}
