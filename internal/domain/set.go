package domain

// QuestionSet は学習モードに対応する問題集合。
type QuestionSet struct {
	ID          int64
	Code        string
	Name        string
	Description string
	TargetSize  int
}

// 既定 3 セットの code。
const (
	SetCodeCore          = "core_300"
	SetCodeJournal       = "journal_240"
	SetCodeComprehensive = "comprehensive_300"
)

// DefaultSetCodes は UI で選択肢に並べる順。
func DefaultSetCodes() []string {
	return []string{SetCodeCore, SetCodeJournal, SetCodeComprehensive}
}
