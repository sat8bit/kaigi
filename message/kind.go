package message

type Kind string

const (
	KindSay    Kind = "say"
	KindSystem Kind = "system"
	KindError  Kind = "error"
)
