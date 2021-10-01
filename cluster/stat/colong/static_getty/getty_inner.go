package static_getty

import (
	getty "github.com/apache/dubbo-getty"
)

type inner interface {
	GetAuthOk(session getty.Session) bool
	SetAuthOk(getty.Session, bool)
}
type innerImpl struct {
}

func newInner() inner {
	return &innerImpl{}
}
func (i *innerImpl) GetAuthOk(session getty.Session) bool {
	ok := session.GetAttribute("auth")
	if ok == nil {
		return false
	}
	return ok.(bool)
}

func (i *innerImpl) SetAuthOk(session getty.Session, b bool) {
	session.SetAttribute("auth", b)
}
