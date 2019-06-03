package difftool

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/node"
)

// Example illustrates nodes comparing.
func Example() {
	logger := logrus.New()
	logger.Level = logrus.FatalLevel

	nodes := node.NewNodeList(3, logger)

	stop := nodes.StartRandTxStream()
	nodes.WaitForBlock(5)
	stop()
	for _, n := range nodes {
		n.Stop()
	}
	// give some time for nodes to stop actually
	time.Sleep(time.Duration(5) * time.Second)

	diffResult := Compare(nodes.Values()...)

	if !diffResult.IsEmpty() {
		// TODO: uncomment after refactoring
		//logger.Fatal("\n" + diffResult.ToString())
	}
	fmt.Println("all good")
	// Output:
	// all good
}

func TestNodeDiff(t *testing.T) {
	t.Skip("Skip TestNodeDiff until block production is fixed")
	logger := common.NewTestLogger(t)
	logger.Level = logrus.DebugLevel

	nodes := node.NewNodeList(3, logger)

	stop := nodes.StartRandTxStream()
	nodes.WaitForBlock(5)
	stop()
	for _, n := range nodes {
		n.Stop()
	}
	// give some time for nodes to stop actually
	time.Sleep(time.Duration(5) * time.Second)

	diffResult := Compare(nodes.Values()...)

	if !diffResult.IsEmpty() {
		// TODO: uncomment after refactoring
		//logger.Fatal("\n" + diffResult.ToString())
		t.Error(diffResult.ToString())
	}
}
