package transaction

import (
	"fmt"
	"io"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// IgnoreList represents a list of transaction hashes to be ignored.
type IgnoreList struct {
	hashes []IgnoredHash // slice of ignored transaction hashes
}

// NewIgnoreList creates a new empty IgnoreList.
func NewIgnoreList() *IgnoreList {
	return &IgnoreList{}
}

// AddProcessedHash adds a record of a transaction hash being associated to a particular transaction in YNAB.
func (i *IgnoreList) AddProcessedHash(transactionHash string, ynabTransactionID string) {
	ignoredHash := IgnoredHash{
		Hash:   transactionHash,
		Reason: fmt.Sprintf("Processed for transaction ID %s on %s", ynabTransactionID, time.Now().Format(time.DateOnly)),
	}
	i.hashes = append(i.hashes, ignoredHash)
}

// AddIgnoredHash adds a transaction hash to the ignore list with a reason.
func (i *IgnoreList) AddIgnoredHash(transactionHash string) {
	ignoredHash := IgnoredHash{
		Hash:   transactionHash,
		Reason: fmt.Sprintf("Marked as ignored on %s", time.Now().Format(time.DateOnly)),
	}
	i.hashes = append(i.hashes, ignoredHash)
}

// IsHashIgnored checks if a transaction hash is in the ignore list.
func (i *IgnoreList) IsHashIgnored(transactionHash string) bool {
	for _, ignoredHash := range i.hashes {
		if strings.EqualFold(ignoredHash.Hash, transactionHash) {
			return true
		}
	}

	return false
}

// GetHashCount returns the number of ignored transaction hashes.
func (i *IgnoreList) GetHashCount() int {
	return len(i.hashes)
}

// GetHashes returns a copy of the list of ignored transaction hashes.
func (i *IgnoreList) GetHashes() []IgnoredHash {
	hashesCopy := make([]IgnoredHash, len(i.hashes))
	copy(hashesCopy, i.hashes)

	return hashesCopy
}

// IgnoredHash represents an ignored transaction hash.
type IgnoredHash struct {
	Hash   string // transaction hash
	Reason string // reason for ignoring the transaction
}

// FromYAML reads an IgnoreList from a YAML representation.
func FromYAML(reader io.Reader) (*IgnoreList, error) {
	var ymlList yamlIgnoreList
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(&ymlList); err != nil {
		return nil, fmt.Errorf("failed to decode ignore list from YAML: %w", err)
	}

	ignoreList := NewIgnoreList()
	for _, ymlHash := range ymlList.IgnoredHashes {
		ignoredHash := &IgnoredHash{
			Hash:   ymlHash.Hash,
			Reason: ymlHash.Reason,
		}
		ignoreList.hashes = append(ignoreList.hashes, *ignoredHash)
	}

	return ignoreList, nil
}

// ToYAML writes an IgnoreList to a YAML representation.
func ToYAML(ignoreList *IgnoreList, writer io.Writer) error {
	var ymlList yamlIgnoreList
	for _, hash := range ignoreList.hashes {
		ymlHash := &yamlIgnoredHash{
			Hash:   hash.Hash,
			Reason: hash.Reason,
		}
		ymlList.IgnoredHashes = append(ymlList.IgnoredHashes, *ymlHash)
	}

	encoder := yaml.NewEncoder(writer)
	defer func() { _ = encoder.Close() }()

	err := encoder.Encode(&ymlList)
	if err != nil {
		return fmt.Errorf("failed to encode ignore list to YAML: %w", err)
	}

	return nil
}

// yamlIgnoreList is an internal struct for YAML serialization.
type yamlIgnoredHash struct {
	Hash   string `yaml:"hash"`   // transaction hash
	Reason string `yaml:"reason"` // reason for ignoring the transaction
}

// FromYAML reads an IgnoreList from a YAML representation.
type yamlIgnoreList struct {
	IgnoredHashes []yamlIgnoredHash `yaml:"ignored_hashes"`
}
