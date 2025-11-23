package http

// LifecycleType represents when a script should be executed
type LifecycleType string

const (
	LifecycleBeforeUser        LifecycleType = "BeforeUser"
	LifecycleBeforeIteration   LifecycleType = "BeforeIteration"
	LifecycleTeardownUser      LifecycleType = "TeardownUser"
	LifecycleTeardownIteration LifecycleType = "TeardownIteration"
	LifecycleNone              LifecycleType = ""
)

// Request represents a single HTTP request
type Request struct {
	Name      string
	Verb      string
	URL       string
	Headers   map[string]string
	Body      string
	PreScript string
	Script    string
	Lifecycle LifecycleType
}
