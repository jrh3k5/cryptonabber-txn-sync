package transaction

const (
	labelTo   = "to"
	labelFrom = "from"
)

// ResolveDirection returns a string label representing the direction of the transaction.
func ResolveDirection(isOutbound bool) string {
	if isOutbound {
		return labelTo
	}

	return labelFrom
}
