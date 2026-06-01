package secondary_key

type SecondaryKeyCreate struct {
	Key string `json:"key"`
}

type SecondaryKeyUpdate struct {
	Key         string `json:"key"`
	LinkedKeyId string `json:"linkedKeyId"`
}
