package citadel

type SecretEntry struct {
	Key   *string
	Value *string
}

type SecretResponse struct {
	Entries   *[]SecretEntry
	PlainText *string
}
