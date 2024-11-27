package iep

import (
	"github.com/mrmikeo/Xpense/inter"
	"github.com/mrmikeo/Xpense/inter/ier"
)

type LlrEpochPack struct {
	Votes  []inter.LlrSignedEpochVote
	Record ier.LlrIdxFullEpochRecord
}
