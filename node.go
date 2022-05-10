package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

type TopologyEntry struct {
	// dst is the MPR selector in the received TCMessage.
	dst NodeID

	// dstMPR is the originator of the TCMessage (last-hop node to the destination).
	dstMPR NodeID

	// msSeqNum is the MPR selector (MS) sequence number, used to determine if a TCMessage contains new information.
	msSeqNum int

	// holdUntil determines how long an entry will be held for before being expelled.
	holdUntil int
}

type RoutingEntry struct {
	// dst is the destination node address (NodeID in this case).
	dst NodeID

	// nextHop is where to send a message to in order to reach the destination.
	nextHop NodeID

	// distance is the number of hops needed to reach the destination.
	distance int
}

type NeighborState int

const (
	Bidirectional NeighborState = iota
	Unidirectional
	MPR
)

type OneHopNeighborEntry struct {
	neighborID NodeID
	state      NeighborState
	holdUntil  int
}

// NodeID is a unique identifier used to differentiate nodes.
type NodeID uint

// Node represents a network node in the ad-hoc network.
type Node struct {
	id NodeID

	// outputLog is where the Node will write all messages that it has sent.
	outputLog io.Writer

	// inputLog is where the Node will write all messages it has received.
	inputLog io.Writer

	// input represents the Node's wireless receiver.
	input <-chan interface{}

	// output represents the Node's wireless transmitter.
	output chan<- interface{}

	// nodeMsg will be sent by the node based on the message's delay.
	nodeMsg NodeMsg

	// topologyTable represents the Node's current perception of the network topology.
	// First NodeID is the desired destination and the second NodeID is an MPR of the destination.
	topologyTable map[NodeID]map[NodeID]TopologyEntry

	// tcSequenceNum is the current TCMessage sequence number.
	tcSequenceNum int

	// topologyHoldTime is how long, in ticks, topology table entries will be held until they are expelled.
	topologyHoldTime int

	routingTable []RoutingEntry

	// oneHopNeighbors is the set of 1-hop neighbors discovered by this node.
	oneHopNeighbors map[NodeID]OneHopNeighborEntry

	// twoHopNeighbors represents the 2-hop neighbors that can be reached via a 1-hop neighbor.
	// The second map is used for uniqueness and merely maps NodeID(s) to themselves.
	twoHopNeighbors map[NodeID]map[NodeID]NodeID

	// msSet
	msSet map[NodeID]NodeID

	// currentTime is the number of ticks since the node came online.
	currentTime int

	// neighborHoldTime is how long, in ticks, neighbor table entries will be held until they are expelled.
	neighborHoldTime int
}

// run starts the Node "listening" for messages.
func (n *Node) run(ctx context.Context) {
	// Continuously listen for new messages until done received by Controller.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	n.currentTime = 0
	for _ = range ticker.C {
		select {
		case <-ctx.Done():
			log.Printf("node %d: recevied done message", n.id)
			return

		case msg := <-n.input:
			_, err := fmt.Fprintln(n.inputLog, msg)
			if err != nil {
				log.Panicf("%d could not write out log: %s", n.id, err)
			}
			log.Printf("node %d: received:\t%s\n", n.id, msg)

			n.handler(msg)
		default:
		}

		if n.currentTime%5 == 0 {
			n.sendHello()
		}
		if n.currentTime%10 == 0 {
			n.sendTC()
		}
		if n.currentTime == n.nodeMsg.delay {
			// send data msg
		}

		// Remove old entries from the neighbor tables.
		for k, entry := range n.oneHopNeighbors {
			if entry.holdUntil <= n.currentTime {
				delete(n.oneHopNeighbors, k)
				delete(n.twoHopNeighbors, k)
			}
		}
		// Remove old entries from the TC tables.
		for _, dst := range n.topologyTable {
			for k, entry := range dst {
				if entry.holdUntil <= n.currentTime {
					delete(dst, k)
				}
			}
		}
		// TODO: Recalculate the routing table, if necessary.

		n.currentTime++
	}
}

func (n *Node) sendHello() {
	// Gather one-hop neighbor entries.
	biNeighbors := make([]NodeID, 0)
	uniNeighbors := make([]NodeID, 0)
	mprNeighbors := make([]NodeID, 0)
	for _, o := range n.oneHopNeighbors {
		switch o.state {
		case Unidirectional:
			uniNeighbors = append(uniNeighbors, o.neighborID)
		case Bidirectional:
			biNeighbors = append(biNeighbors, o.neighborID)
		case MPR:
			mprNeighbors = append(mprNeighbors, o.neighborID)
		default:
			log.Panicf("node %d: invalid one-hop neighbor type: %d", n.id, o.state)
		}
	}

	hello := &HelloMessage{
		src:    n.id,
		unidir: uniNeighbors,
		bidir:  biNeighbors,
		mpr:    mprNeighbors,
	}
	n.output <- hello
	log.Printf("node %d: sent:\t%s", n.id, hello)
}

func (n *Node) sendTC() {
	// Get the MS set node IDs to include in the TC message.
	msSet := make([]NodeID, 0)
	for _, id := range n.msSet {
		msSet = append(msSet, id)
	}

	tc := &TCMessage{
		src:     n.id,
		fromnbr: n.id,
		seq:     n.tcSequenceNum,
		ms:      msSet,
	}
	n.output <- tc
	log.Printf("node %d: sent:\t%s", n.id, tc)

	n.tcSequenceNum++
}

// handler de-multiplexes messages to their respective handlers.
func (n *Node) handler(msg interface{}) {
	switch t := msg.(type) {
	case *HelloMessage:
		n.handleHello(msg.(*HelloMessage))
	case *DataMessage:
		n.handleData(msg.(*DataMessage))
	case *TCMessage:
		n.handleTC(msg.(*TCMessage))
	default:
		log.Panicf("node %d: invalid message type: %s\n", n.id, t)
	}
}

// updateOneHopNeighbors adds all new one-hop neighbors that can be reached.
func updateOneHopNeighbors(msg *HelloMessage, oneHopNeighbors map[NodeID]OneHopNeighborEntry, holdUntil int, id NodeID) map[NodeID]OneHopNeighborEntry {
	entry, ok := oneHopNeighbors[msg.src]
	if !ok {
		// First time neighbor
		oneHopNeighbors[msg.src] = OneHopNeighborEntry{
			neighborID: msg.src,
			state:      Unidirectional,
			holdUntil:  holdUntil,
		}
	} else {
		// Already unidirectional neighbor
		entry.holdUntil = holdUntil

		// Check if the link state should be updated.
		for _, nodeID := range append(msg.unidir, append(msg.bidir, msg.mpr...)...) {
			if nodeID == id {
				entry.state = Bidirectional
				break
			}
		}

		oneHopNeighbors[msg.src] = entry
	}
	return oneHopNeighbors
}

// updateTwoHopNeighbors adds all new two-hop neighbors that can be reached.
func updateTwoHopNeighbors(msg *HelloMessage, twoHopNeighbors map[NodeID]map[NodeID]NodeID, id NodeID) map[NodeID]map[NodeID]NodeID {
	// Delete all previous entries for the source by creating a new map.
	twoHops := make(map[NodeID]NodeID)
	for _, nodeID := range append(msg.unidir, msg.bidir...) {
		// Check for own id.
		if nodeID == id {
			continue
		}
		twoHops[nodeID] = nodeID
	}
	twoHopNeighbors[msg.src] = twoHops
	return twoHopNeighbors
}

// calculateMPRs creates a new MPR set based on the current neighbor tables.
func calculateMPRs(oneHopNeighbors map[NodeID]OneHopNeighborEntry, twoHopNeighbors map[NodeID]map[NodeID]NodeID) map[NodeID]OneHopNeighborEntry {
	// Copy one hop neighbors
	remainingTwoHops := make(map[NodeID]NodeID)
	nodes := make([]NodeID, 0)
	for node, v := range twoHopNeighbors {
		// Only consider nodes as MPRs if they are bidirectional.
		ohn, _ := oneHopNeighbors[node]
		if ohn.state == Unidirectional {
			continue
		}
		nodes = append(nodes, node)
		for k, _ := range v {
			remainingTwoHops[k] = k
		}
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})

	// Set of MPRs
	mprs := make(map[NodeID]NodeID)

	for len(remainingTwoHops) > 0 {
		maxTwoHopsID := nodes[0]
		nodes = nodes[1:]

		mprs[maxTwoHopsID] = maxTwoHopsID

		for k, _ := range twoHopNeighbors[maxTwoHopsID] {
			delete(remainingTwoHops, k)
		}
	}

	// Update states of one-hop neighbors based on newly selected MPRs.
	for id, neigh := range oneHopNeighbors {
		_, ok := mprs[id]
		if ok {
			neigh.state = MPR
			oneHopNeighbors[id] = neigh
		} else {
			if neigh.state == MPR {
				neigh.state = Bidirectional
				oneHopNeighbors[id] = neigh
			}
		}
	}
	return oneHopNeighbors
}

// handleHello handles the processing of a HelloMessage.
func (n *Node) handleHello(msg *HelloMessage) {
	// Update one-hop neighbors.
	n.oneHopNeighbors = updateOneHopNeighbors(msg, n.oneHopNeighbors, n.currentTime+n.neighborHoldTime, n.id)

	// Update two-hop neighbors
	n.twoHopNeighbors = updateTwoHopNeighbors(msg, n.twoHopNeighbors, n.id)

	n.oneHopNeighbors = calculateMPRs(n.oneHopNeighbors, n.twoHopNeighbors)

	// Update the msSet
	_, ok := n.msSet[msg.src]
	isMS := false
	// Check if this node is in the MPR set from the HELLO message.
	for _, nodeID := range msg.mpr {
		if nodeID == n.id {
			isMS = true
			break
		}
	}
	// Previously an MS, but no longer are.
	if ok && !isMS {
		delete(n.msSet, msg.src)
	}
	// New MS.
	if !ok && isMS {
		n.msSet[msg.src] = msg.src
	}
}

func (n *Node) handleData(msg *DataMessage) {
	fmt.Printf("node %d: received message of type: %s\n", n.id, DataType)
}

func updateTopologyTable(msg *TCMessage, topologyTable map[NodeID]map[NodeID]TopologyEntry, holdUntil int) map[NodeID]map[NodeID]TopologyEntry {
	for _, dst := range msg.ms {
		entries, ok := topologyTable[dst]
		if !ok {
			// First time seeing this destination
			entries = make(map[NodeID]TopologyEntry)
			entries[msg.src] = TopologyEntry{
				dst:       dst,
				dstMPR:    msg.src,
				msSeqNum:  msg.seq,
				holdUntil: holdUntil,
			}
			topologyTable[dst] = entries
			continue
		}

		entry, ok := entries[msg.src]
		if !ok {
			// First time seeing this MPR for the destination.
			entries[msg.src] = TopologyEntry{
				dst:       dst,
				dstMPR:    msg.src,
				msSeqNum:  msg.seq,
				holdUntil: holdUntil,
			}
			continue
		}

		// We've seen this (dst, mpr) pair already.
		if entry.msSeqNum < msg.seq {
			entry.holdUntil = holdUntil
			entries[msg.src] = entry
		}
	}

	return topologyTable
}

func (n *Node) handleTC(msg *TCMessage) {
	// Ignore TC messages sent by this node.
	if msg.src == n.id {
		return
	}

	n.topologyTable = updateTopologyTable(msg, n.topologyTable, n.currentTime+n.topologyHoldTime)

	// Update the from-neighbor field.
	msg.fromnbr = n.id

	// Send the updated msg.
	n.output <- msg

	log.Printf("node %d: sent:\t\t%s", n.id, msg)
}

type NodeMsg struct {
	msg   string
	delay int
	dst   NodeID
}

// NewNode creates a network Node.
func NewNode(input <-chan interface{}, output chan<- interface{}, id NodeID, nodeMsg NodeMsg) *Node {
	n := Node{}
	n.id = id
	n.input = input
	n.output = output
	n.nodeMsg = nodeMsg
	n.inputLog = ioutil.Discard
	n.outputLog = ioutil.Discard
	n.topologyTable = make(map[NodeID]map[NodeID]TopologyEntry)
	n.oneHopNeighbors = make(map[NodeID]OneHopNeighborEntry)
	n.twoHopNeighbors = make(map[NodeID]map[NodeID]NodeID)
	n.msSet = make(map[NodeID]NodeID)
	n.neighborHoldTime = 15
	return &n
}
