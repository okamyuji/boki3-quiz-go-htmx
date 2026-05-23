package domain

import "time"

// QuestionType は出題形式を表す。Spec 5.2 の question_type 列に対応。
type QuestionType string

// QuestionType の列挙値。日商簿記 3 級の出題形式に対応する。
const (
	QuestionTypeJournal      QuestionType = "journal"       // 仕訳
	QuestionTypeLedger       QuestionType = "ledger"        // 勘定記入
	QuestionTypeSubbook      QuestionType = "subbook"       // 補助簿
	QuestionTypeTrialBalance QuestionType = "trial_balance" // 試算表
	QuestionTypeWorksheet    QuestionType = "worksheet"     // 精算表
	QuestionTypeFS           QuestionType = "fs"            // 財務諸表
	QuestionTypeSlip         QuestionType = "slip"          // 伝票
)

// IsValid は既知の QuestionType か判定する。
func (q QuestionType) IsValid() bool {
	switch q {
	case QuestionTypeJournal, QuestionTypeLedger, QuestionTypeSubbook,
		QuestionTypeTrialBalance, QuestionTypeWorksheet, QuestionTypeFS, QuestionTypeSlip:
		return true
	}
	return false
}

// QuizMode は次問題抽選戦略を表す。
type QuizMode string

// QuizMode の列挙値。
const (
	QuizModeSRS        QuizMode = "srs"
	QuizModeSequential QuizMode = "sequential"
	QuizModeRandom     QuizMode = "random"
)

// IsValid は既知の QuizMode か判定する。
func (m QuizMode) IsValid() bool {
	switch m {
	case QuizModeSRS, QuizModeSequential, QuizModeRandom:
		return true
	}
	return false
}

// JournalEntry は仕訳 1 行を表す (借方 or 貸方)。
type JournalEntry struct {
	Account string // 勘定科目
	Amount  int64  // 円 (整数のみ)
}

// AnswerPayload は採点対象の解答ペイロード。問題形式により Journal / Choice / Text を使い分ける。
type AnswerPayload struct {
	Type    QuestionType
	Debits  []JournalEntry // journal 用
	Credits []JournalEntry // journal 用
	Choice  string         // multiple choice や穴埋め用
	Text    string         // 自由記述用 (未使用、将来拡張)
}

// Question は問題マスタの 1 件。payload_json / answer_json は JSON 文字列のまま保持し、
// repo 層で AnswerPayload へデコードする。
type Question struct {
	ID             int64
	Code           string
	TopicID        int64
	QuestionType   QuestionType
	Difficulty     int
	Prompt         string
	PayloadJSON    string
	AnswerJSON     string
	Explanation    string
	ReferencesJSON string
	CreatedAt      time.Time
}

// QuestionFilter は QuestionRepository.Search の引数。
type QuestionFilter struct {
	TopicCodes []string
	Types      []QuestionType
	SetCode    string
	Limit      int
	Offset     int
}
