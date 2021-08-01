package sessions

func SessionInit(session string) {
	switch session {
	default:
		memProviderInit()
	}
}
