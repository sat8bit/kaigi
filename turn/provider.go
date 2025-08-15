package turn

// TurnProvider は、現在のターン情報を提供します。
// これにより、他のコンポーネントは Supervisor のような具体的な実装を知ることなく、
// ターン情報にアクセスできます。
type TurnProvider interface {
	GetCurrentTurn() int
	GetMaxTurns() int
}
