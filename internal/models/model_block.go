package models

import "time"

type Block struct {
	Index       int
	Hash        []byte
	Transaction *Transaction
	PrevHash    []byte
	Timestamp   time.Time
	HMAC        []byte
}
