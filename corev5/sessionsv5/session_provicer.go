package sessionsv5

type SessionsProvider interface {
	New(id string, cleanStart bool) (Session, error)
	Get(id string, cleanStart bool) (Session, error)
	Del(id string)
	Save(id string) error
	Count() int
	Close() error
}
