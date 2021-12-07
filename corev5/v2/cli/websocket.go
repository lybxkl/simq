package cli

func DefaultListenAndServeWebsocket() error {
	if err := AddWebsocketHandler("/mqtt", "127.0.0.1:1883"); err != nil {
		return err
	}
	return ListenAndServeWebsocket(":1234")
}
