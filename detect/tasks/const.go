package tasks

import "time"

// timout for various network operations
const timeout = 200 * time.Millisecond

// success is a single nil result indicating that the task was successful
var success = []interface{}{nil}
