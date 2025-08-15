package topic

// Topic は、会話のきっかけとなる、構造化された「話題」を表します。
// この構造体は、話題の出所（RSS、APIなど）に依存しない、汎用的な形式です。
type Topic struct {
	// Title は、話題のタイトルや見出しです。
	Title string

	// Summary は、話題の短い要約です。
	Summary string

	// SourceURL は、話題の出所を示すURLです。
	SourceURL string
}
