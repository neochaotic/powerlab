package service

import "time"

// realNowUnix isolates the time.Now() call so the test file can stub
// nowUnix without touching production logic. Defined in its own file
// to make the seam obvious.
func realNowUnix() int64 { return time.Now().Unix() }
