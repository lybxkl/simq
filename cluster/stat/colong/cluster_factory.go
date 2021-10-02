package colong

type NodeServerFace interface {
	Close() error
}
type NodeClientFace interface {
	Close() error
}
