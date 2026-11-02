package ui

import "time"

// undoWindow is how long a deletion can be undone: the toast's life.
const undoWindow = 6 * time.Second

// afterToast fires once an undo toast has gone.
func afterToast() <-chan time.Time { return time.After(undoWindow) }
