## Issue Tracking

We use `bd` (beads) for task tracking. No markdown TODOs.

```bash
bd ready --json                                    # see what's ready
bd create "title" -t bug|feature|task -p 0-4       # new issue
bd update <id> --status in_progress                # claim it
bd close <id> --reason "done"                      # finish it
```

Priority: 0=critical, 1=high, 2=medium, 3=low, 4=backlog

Store planning docs in `history/` not repo root.
