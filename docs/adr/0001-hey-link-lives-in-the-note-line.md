# The HEY Link is a tag on the Task line

Pike needs to know which HEY Todo a Task corresponds to, and tasks in markdown have no stable identity: lines move between files, get reordered, and get reworded. We record the Link as an `@hey(id)` tag written into the Task's own line, because that is the only thing that survives all three kinds of edit. A sidecar keyed by file and line breaks on reordering; one keyed by a text hash breaks on rewording.

## Considered options

- Sidecar state file keyed by file+line: invisible to the user, but any edit above the task silently unlinks it.
- Sidecar keyed by a hash of the task text: survives moves, but a typo fix creates a duplicate Todo.
- Tag in the line (chosen): visible in the notes, but the toggler already rewrites lines atomically and every other piece of pike metadata (`@due`, `@completed`, `@hidden`) already lives this way.

## Consequences

Users will see HEY ids in their notes unless the renderer hides them. Removing the tag by hand is the supported way to unlink a Task.
