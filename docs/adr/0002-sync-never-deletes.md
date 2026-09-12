# Sync never deletes on either side

When one side of a Link disappears, pike does not delete the other. A Todo gone from HEY leaves the Task in place with its `@hey` tag stripped. A Task line gone from the notes leaves the Todo in HEY and reports an Orphan Warning.

The reason is that "the line is gone" is indistinguishable from a file being excluded by a glob, moved outside the notes directory, or temporarily unparseable, and "the Todo is gone" is indistinguishable from a wrong account or an expired session. Neither signal is trustworthy enough to destroy user data over. The cost is that deletions must be done by hand on both sides, which we accept because deletion is rare and loss is unrecoverable.
