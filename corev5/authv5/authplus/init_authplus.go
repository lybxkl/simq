package authplus

func InitAuthPlus(allows []string) {
	for _, allow := range allows {
		switch allow {
		default:
			defaultAuthPlusInit()
		}
	}
}
