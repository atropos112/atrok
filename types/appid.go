package types

type AppId interface {
	ID() string
	GetSpecHash() (string, error)
	GetLastReconciliation() (bool, string)
}
