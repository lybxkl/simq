package sessionsv5

type SessionsProvider interface {
	New(id string, cleanStart bool, expiryTime uint32) (Session, error)
	Get(id string, cleanStart bool, expiryTime uint32) (Session, error)
	Del(id string)
	Save(id string) error
	Count() int
	Close() error
}
