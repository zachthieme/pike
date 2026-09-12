# Pike

Pike is a terminal task dashboard over a directory of markdown notes. The notes are the primary home of every task; external systems, such as HEY, are mirrors that pike keeps in step.

## Language

### Notes

**Task**:
A single markdown line in the notes that pike treats as a work item, either a checkbox line or a tagged bullet. A Task is identified by its file and line.
_Avoid_: item, todo (that word is reserved for HEY)

**Tag**:
An `@name` or `@name(value)` token on a Task line. Tags carry all of pike's metadata: due dates, completion dates, hiding, and links to external systems.

**Inbox**:
The single configured notes file where captured Tasks land before being filed elsewhere.
_Avoid_: default file, capture file

**Warning**:
A non-fatal problem pike reports to the user without stopping, such as an unparseable line or an unreachable external system.

### HEY integration

**Todo**:
A to-do item in HEY's calendar. It has an id, a title, a Week, and optionally a completion time. It carries no tags, no hierarchy, and no day-level date.
_Avoid_: HEY task, event

**Week**:
The Sunday-to-Saturday span a Todo is filed under. HEY snaps any date it is given to the Week containing it, so a Todo never knows which day within its Week it was meant for.
_Avoid_: date, due date (those belong to Tasks)

**Link**:
The association between exactly one Task and exactly one Todo, recorded as an `@hey(id)` Tag on the Task line. A Link is sticky: once made, it persists whether or not the Task still matches the Sync Query.
_Avoid_: mapping, binding, pairing

**Linked Task** / **Linked Todo**:
A Task or Todo that is one side of a Link. An unlinked Task is local-only; an unlinked Todo is HEY-only until Sync imports it.

**Sync Query**:
The configured pike query that selects which Eligible Tasks are pushed to HEY as new Todos. It governs Link creation only, never Link removal.
_Avoid_: filter, scope

**Eligible Task**:
An unlinked Task that is open, has a checkbox, and is not hidden. Only Eligible Tasks matching the Sync Query become Linked; plain bullets and hidden Tasks never do.

**Re-create**:
Replacing a Linked Todo with a fresh one because its Title or Week changed in the notes and HEY offers no way to edit a Todo in place. The Link moves to the new Todo; only open Todos are ever Re-created.
_Avoid_: rename, update, edit

**Sync**:
The reconciliation pass that brings every Link's two sides into agreement and creates new Links in both directions: eligible Tasks become Todos, unlinked open Todos become Tasks in the Inbox.
_Avoid_: import, export, mirror (each names only half of it)

**Push**:
Sending a single Task change to its Linked Todo immediately, outside a full Sync. Happens when a Task is completed or uncompleted in the TUI.

**Title**:
The text of a Todo. It is the Task's text with every Tag removed; Tags never travel to HEY.

**Orphan**:
A Link whose other side can no longer be found: a Linked Todo deleted in HEY, or a Linked Task whose line has gone from the notes. Pike reports Orphans as Warnings and never deletes the surviving side.
_Avoid_: dangling link, stale link
