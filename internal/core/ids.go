package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// idLen is how many hex characters of the SHA-256 digest an entity ID carries.
// 24 hex chars = 96 bits, ample to avoid collisions while staying readable.
const idLen = 24

// DeriveConversationID computes the deterministic ID for a conversation from
// its (agent, sessionID) key. Deterministic IDs make EnsureConversation
// idempotent without a round-trip to read back a generated key.
func DeriveConversationID(agent Agent, sessionID string) string {
	return hashID("conv", string(agent), sessionID)
}

// DeriveMessageID computes the deterministic ID for a message from its
// conversation and identity hash, so re-ingesting the same source line yields
// the same ID.
func DeriveMessageID(conversationID, idHash string) string {
	return hashID("msg", conversationID, idHash)
}

// DeriveSummaryID computes a deterministic ID for a summary from its
// conversation, kind, depth, and ordered source IDs, so re-summarizing the same
// span yields the same node (idempotent DAG construction).
func DeriveSummaryID(conversationID string, kind SummaryKind, depth int, sourceIDs []string) string {
	parts := make([]string, 0, 3+len(sourceIDs))
	parts = append(parts, conversationID, string(kind), strconv.Itoa(depth))
	parts = append(parts, sourceIDs...)
	return hashID("sum", parts...)
}

// DeriveFileID computes a deterministic ID for an offloaded large file from its
// conversation and source message.
func DeriveFileID(conversationID, messageID string) string {
	return hashID("file", conversationID, messageID)
}

// IdentityHash is the stable fingerprint used to dedupe messages within a
// conversation. When the source provides an external ID (an agent message or
// transcript-line ID) it is authoritative; otherwise the role+content is
// hashed so identical re-emitted turns collapse.
func IdentityHash(externalID, role, content string) string {
	if externalID != "" {
		return digest("ext", externalID)
	}
	return digest(role, content)
}

func hashID(prefix string, parts ...string) string {
	return prefix + "_" + digest(parts...)[:idLen]
}

func digest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
