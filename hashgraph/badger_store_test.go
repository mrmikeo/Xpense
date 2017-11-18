package hashgraph

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/babbleio/babble/crypto"
)

func initBadgerStore(cacheSize int, t *testing.T) (*BadgerStore, []pub) {
	n := 3
	participantPubs := []pub{}
	participants := make(map[string]int)
	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateECDSAKey()
		pubKey := crypto.FromECDSAPub(&key.PublicKey)
		participantPubs = append(participantPubs,
			pub{i, pubKey, fmt.Sprintf("0x%X", pubKey)})
		participants[fmt.Sprintf("0x%X", pubKey)] = i
	}

	dir, err := ioutil.TempDir("test_data", "badger")
	if err != nil {
		log.Fatal(err)
	}

	store, err := NewBadgerStore(participants, cacheSize, dir)
	if err != nil {
		t.Fatal(err)
	}

	return store, participantPubs
}

func removeBadgerStore(store *BadgerStore, t *testing.T) {
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(store.path); err != nil {
		t.Fatal(err)
	}
}

func TestNewBadgerStore(t *testing.T) {
	dir, err := ioutil.TempDir("test_data", "badger")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	participants := map[string]int{
		"alice":   0,
		"bob":     1,
		"charlie": 2,
	}
	cacheSize := 100

	store, err := NewBadgerStore(participants, cacheSize, dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if store.path != dir {
		t.Fatalf("unexpected path %q", store.path)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("err: %s", err)
	}

	//check roots
	inmemRoots := store.inmemStore.roots
	for participant, root := range inmemRoots {
		dbRoot, err := store.dbGetRoot(participant)
		if err != nil {
			t.Fatalf("Error retrieving DB root for participant %s: %s", participant, err)
		}
		if !reflect.DeepEqual(dbRoot, root) {
			t.Fatalf("%s DB root should be %#v, not %#v", participant, root, dbRoot)
		}
	}

	if err := store.Close(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

//Call DB methods directly
func TestDBMethods(t *testing.T) {
	cacheSize := 10
	testSize := 100
	store, participants := initBadgerStore(cacheSize, t)
	defer removeBadgerStore(store, t)

	//inset events in db directly
	events := make(map[string][]Event)
	for _, p := range participants {
		items := []Event{}
		for k := 0; k < testSize; k++ {
			event := NewEvent([][]byte{[]byte(fmt.Sprintf("%s_%d", p.hex[:5], k))},
				[]string{"", ""},
				p.pubKey,
				k)
			items = append(items, event)
			err := store.dbSetEvents([]Event{event})
			if err != nil {
				t.Fatal(err)
			}
		}
		events[p.hex] = items
	}

	//check events where correctly inserted and can be retrieved
	for p, evs := range events {
		for k, ev := range evs {
			rev, err := store.dbGetEvent(ev.Hex())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ev.Body, rev.Body) {
				t.Fatalf("events[%s][%d].Body should be %#v, not %#v", p, k, ev, rev)
			}
			if !reflect.DeepEqual(ev.S, rev.S) {
				t.Fatalf("events[%s][%d].S should be %#v, not %#v", p, k, ev.S, rev.S)
			}
			if !reflect.DeepEqual(ev.R, rev.R) {
				t.Fatalf("events[%s][%d].R should be %#v, not %#v", p, k, ev.R, rev.R)
			}
		}
	}

	//check that participant events where correctly added
	skipIndex := -1 //do not skip any indexes
	for _, p := range participants {
		pEvents, err := store.dbParticipantEvents(p.hex, skipIndex)
		if err != nil {
			t.Fatal(err)
		}
		if l := len(pEvents); l != testSize {
			t.Fatalf("%s should have %d events, not %d", p.hex, testSize, l)
		}

		expectedEvents := events[p.hex][skipIndex+1:]
		for k, e := range expectedEvents {
			if e.Hex() != pEvents[k] {
				t.Fatalf("ParticipantEvents[%s][%d] should be %s, not %s",
					p.hex, k, e.Hex(), pEvents[k])
			}
		}
	}

	//check that partipant last was correctly added
	for _, p := range participants {
		last, err := store.dbGetLastFrom(p.hex)
		if err != nil {
			t.Fatal(err)
		}

		evs := events[p.hex]
		expectedLast := evs[len(evs)-1]
		if last != expectedLast.Hex() {
			t.Fatalf("%s last should be %s, not %s", p.hex, expectedLast.Hex(), last)
		}
	}
}

//Check that the wrapper methods work
//These methods use the inmemStore as a cache on top of the DB
func TestBadgerEvents(t *testing.T) {
	//Insert more events than can fit in cache to test retrieving from db.
	cacheSize := 10
	testSize := 100
	store, participants := initBadgerStore(cacheSize, t)
	defer removeBadgerStore(store, t)

	//insert event
	events := make(map[string][]Event)
	for _, p := range participants {
		items := []Event{}
		for k := 0; k < testSize; k++ {
			event := NewEvent([][]byte{[]byte(fmt.Sprintf("%s_%d", p.hex[:5], k))},
				[]string{"", ""},
				p.pubKey,
				k)
			items = append(items, event)
			err := store.SetEvent(event)
			if err != nil {
				t.Fatal(err)
			}
		}
		events[p.hex] = items
	}

	// check that events were correclty inserted
	for p, evs := range events {
		for k, ev := range evs {
			rev, err := store.GetEvent(ev.Hex())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ev.Body, rev.Body) {
				t.Fatalf("events[%s][%d].Body should be %#v, not %#v", p, k, ev, rev)
			}
			if !reflect.DeepEqual(ev.S, rev.S) {
				t.Fatalf("events[%s][%d].S should be %#v, not %#v", p, k, ev.S, rev.S)
			}
			if !reflect.DeepEqual(ev.R, rev.R) {
				t.Fatalf("events[%s][%d].R should be %#v, not %#v", p, k, ev.R, rev.R)
			}
		}
	}

	//check retrieving events per participant
	skipIndex := -1 //do not skip any indexes
	for _, p := range participants {
		pEvents, err := store.ParticipantEvents(p.hex, skipIndex)
		if err != nil {
			t.Fatal(err)
		}
		if l := len(pEvents); l != testSize {
			t.Fatalf("%s should have %d events, not %d", p.hex, testSize, l)
		}

		expectedEvents := events[p.hex][skipIndex+1:]
		for k, e := range expectedEvents {
			if e.Hex() != pEvents[k] {
				t.Fatalf("ParticipantEvents[%s][%d] should be %s, not %s",
					p.hex, k, e.Hex(), pEvents[k])
			}
		}
	}

	//check retrieving participant last
	for _, p := range participants {
		last, _, err := store.LastFrom(p.hex)
		if err != nil {
			t.Fatal(err)
		}

		evs := events[p.hex]
		expectedLast := evs[len(evs)-1]
		if last != expectedLast.Hex() {
			t.Fatalf("%s last should be %s, not %s", p.hex, expectedLast.Hex(), last)
		}
	}

	expectedKnown := make(map[int]int)
	for _, p := range participants {
		expectedKnown[p.id] = testSize - 1
	}
	known := store.Known()
	if !reflect.DeepEqual(expectedKnown, known) {
		t.Fatalf("Incorrect Known. Got %#v, expected %#v", known, expectedKnown)
	}

	for _, p := range participants {
		evs := events[p.hex]
		for _, ev := range evs {
			if err := store.AddConsensusEvent(ev.Hex()); err != nil {
				t.Fatal(err)
			}
		}

	}
}
