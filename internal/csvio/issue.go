package csvio

type IssueCode string

const (
	IssueDuplicateKey IssueCode = "duplicate_key"

	IssueShortRow IssueCode = "short_row"

	IssueLongRow IssueCode = "long_row"
)

type Severity string

const (
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

type Issue struct {
	Code     IssueCode `json:"code"`
	Severity Severity  `json:"severity"`
	Line     int       `json:"line"`
	Key      string    `json:"key"`
	Detail   string    `json:"detail"`
}
