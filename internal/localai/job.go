package localai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobDone      JobStatus = "done"
	JobCancelled JobStatus = "cancelled"
)

type JobKind string

const (
	JobKindTranslate JobKind = "translate"
	JobKindDiscover  JobKind = "discover"
)

const jobRecentCap = 20

type Job struct {
	ID       string
	Kind     JobKind
	FileType string
	Langs    []string
	Priority bool
	ctx      context.Context
	cancel   context.CancelFunc

	mu              sync.Mutex
	status          JobStatus
	total           int
	done            int
	skipped         int
	errored         int
	discovered      int
	discoveryErrors int
	recent          []string
	lastError       string
	lastSkipped     string
	startedAt       time.Time
	finishedAt      time.Time
}

func newJobID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func (j *Job) recordDone(key string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done++
	j.pushRecent(key)
}

func (j *Job) recordSkipped(key, reason string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.skipped++
	j.lastSkipped = key + ": " + reason
}

func (j *Job) recordError(key string, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.errored++
	j.lastError = key + ": " + err.Error()
}

func (j *Job) recordDiscovered(n int) {
	if n == 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.discovered += n
}

func (j *Job) recordDiscoveryError(key string, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.discoveryErrors++
	j.lastError = key + " (discovery): " + err.Error()
}

func (j *Job) pushRecent(key string) {
	j.recent = append(j.recent, key)
	if len(j.recent) > jobRecentCap {
		j.recent = j.recent[len(j.recent)-jobRecentCap:]
	}
}

func (j *Job) finish(status JobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != JobRunning {
		return
	}
	j.status = status
	j.finishedAt = time.Now()
}

func (j *Job) cancelled() bool {
	return j.ctx.Err() != nil
}

type JobStatusResponse struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"`
	FileType        string     `json:"file_type"`
	Langs           []string   `json:"langs"`
	Priority        bool       `json:"priority"`
	Status          string     `json:"status"`
	Total           int        `json:"total"`
	Done            int        `json:"done"`
	Skipped         int        `json:"skipped"`
	Errored         int        `json:"errored"`
	Discovered      int        `json:"discovered"`
	DiscoveryErrors int        `json:"discovery_errors"`
	Recent          []string   `json:"recent"`
	LastError       string     `json:"last_error,omitempty"`
	LastSkipped     string     `json:"last_skipped,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

func (j *Job) snapshot() JobStatusResponse {
	j.mu.Lock()
	defer j.mu.Unlock()
	resp := JobStatusResponse{
		ID:              j.ID,
		Kind:            string(j.Kind),
		FileType:        j.FileType,
		Langs:           append(make([]string, 0, len(j.Langs)), j.Langs...),
		Priority:        j.Priority,
		Status:          string(j.status),
		Total:           j.total,
		Done:            j.done,
		Skipped:         j.skipped,
		Errored:         j.errored,
		Discovered:      j.discovered,
		DiscoveryErrors: j.discoveryErrors,
		Recent:          append([]string(nil), j.recent...),
		LastError:       j.lastError,
		LastSkipped:     j.lastSkipped,
		StartedAt:       j.startedAt,
	}
	if !j.finishedAt.IsZero() {
		t := j.finishedAt
		resp.FinishedAt = &t
	}
	return resp
}

type Manager struct {
	mu    sync.Mutex
	jobs  map[string]*Job
	queue queue
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) addJob(j *Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs == nil {
		m.jobs = map[string]*Job{}
	}
	m.jobs[j.ID] = j
}

func (m *Manager) GetJob(id string) (JobStatusResponse, bool) {
	j, ok := m.lookupJob(id)
	if !ok {
		return JobStatusResponse{}, false
	}
	return j.snapshot(), true
}

func (m *Manager) lookupJob(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

func (m *Manager) listJobs(fileType string) []*Job {
	m.mu.Lock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if j.FileType == fileType {
			jobs = append(jobs, j)
		}
	}
	m.mu.Unlock()
	sort.Slice(jobs, func(i, k int) bool { return jobs[i].startedAt.Before(jobs[k].startedAt) })
	return jobs
}
