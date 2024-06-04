package models

type VerificationMechanism string

// BlockChain is the type for blockchains. Uniquely defined by its @Name.
type Blockchain struct {
	Name        string `json:"Name"`
	NativeToken Asset  `json:"NativeToken"`
	// Verificationmechanism is in short notation, such as pos for proof-of-stake
	VerificationMechanism VerificationMechanism `json:"VerificationMechanism"`
	// ChainID refers to EVM based chains and is thereby optional.
	ChainID string `json:"ChainID"`
}
