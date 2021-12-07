package auth

type noAuthenticator bool

var _ Authenticator = (*noAuthenticator)(nil)

var (
	noAuth noAuthenticator = true
)

func NewDefaultAuth() Authenticator {
	return &noAuth
}

//权限认证
func (this noAuthenticator) Authenticate(id string, cred interface{}) error {
	return nil
}
