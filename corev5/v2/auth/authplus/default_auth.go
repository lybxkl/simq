package authplus

type defaultAuth struct {
	i int
}

func NewDefaultAuth() AuthPlus {
	return &defaultAuth{}
}
func (d2 *defaultAuth) Verify(authData []byte) (d []byte, continueAuth bool, err error) {
	if d2.i < 5 {
		d2.i++
		return []byte("continue"), true, nil
	} else {
		d2.i = 0
		return nil, false, nil
	}
}
