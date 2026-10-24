package localize

var KnownLanguages = []string{
	"Deutsch", "Français", "Italiano", "Spanish", "Portuguese (Euro)",
	"Portuguese (Brazil)", "Polish", "Russian", "Japanese",
	"Chinese (simplified)", "Chinese (traditional)", "Korean", "Turkish",
	"Greek", "Dutch", "Vietnamese",
}

type FileType string

const (
	FileTypeLocalization FileType = "Localization"
	FileTypeDialogues    FileType = "Dialogues"
	FileTypePDA          FileType = "PDA"

	FileTypeLocalizationVanilla FileType = "LocalizationVanilla"
)

func (ft FileType) Valid() bool {
	switch ft {
	case FileTypeLocalization, FileTypeDialogues, FileTypePDA, FileTypeLocalizationVanilla:
		return true
	default:
		return false
	}
}

func (ft FileType) SupportsChains() bool {
	return ft == FileTypeDialogues || ft == FileTypePDA
}

func (ft FileType) ExportFilename() string {
	if ft == FileTypeLocalizationVanilla {
		return string(FileTypeLocalization) + ".csv"
	}
	return string(ft) + ".csv"
}

type Header struct {
	KeyColumn string
	Languages []string
}

type Row struct {
	Key    string
	Values map[string]string
	Line   int
}

func (r Row) Value(lang string) string {
	if r.Values == nil {
		return ""
	}
	return r.Values[lang]
}

type Table struct {
	Header Header
	Rows   []Row
}
