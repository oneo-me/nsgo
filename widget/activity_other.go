//go:build !darwin && !android

package widget

func startActivity(*Activity, ActivityContent, ActivityOptions) error {
	return ErrActivitiesUnsupported
}

func updateActivity(*Activity, ActivityContent, *activityAlert) error {
	return ErrActivitiesUnsupported
}

func endActivity(*Activity, *ActivityContent, Dismissal) error { return ErrActivitiesUnsupported }
func runningActivities() []*Activity                           { return nil }
func activitiesEnabled() bool                                  { return false }
func listen()                                                  {}
