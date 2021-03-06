package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

// QueryMsg enables the Controller to query the NetworkTopology to determine the state of a link at a given moment
// in time.
type QueryMsg struct {
	// FromNode is the source of the link.
	FromNode NodeID

	// ToNode is the destination of the link.
	ToNode NodeID

	// AtTime is the moment in time to check the status of the link.
	AtTime int
}

// NetworkTypology represents the ad-hoc network typology and is used by the Controller.
type NetworkTypology struct {
	links map[NodeID]map[NodeID]Link
}

type ErrParseLinkState struct {
	msg string
}

func (e ErrParseLinkState) Error() string {
	return fmt.Sprintf("parse link state: %s", e.msg)
}

func NewNetworkTypology(in io.Reader) (*NetworkTypology, error) {
	n := &NetworkTypology{}
	n.links = make(map[NodeID]map[NodeID]Link)

	r := bufio.NewReader(in)
	currTime := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		line = strings.TrimSuffix(line, "\n")

		ls, err := parseLinkState(line)
		if err != nil {
			log.Fatalln(err)
		}

		if ls.time < currTime {
			return nil, errors.New("entries in input must be sorted by increasing time")
		}
		currTime = ls.time

		// Add the new LinkState to the applicable link. If there is not a link, create one.
		dsts, in := n.links[ls.fromNode]
		if !in {
			link := Link{fromNode: ls.fromNode, toNode: ls.toNode}
			link.states = append(link.states, *ls)

			srcMap := make(map[NodeID]Link)
			srcMap[ls.toNode] = link
			n.links[ls.fromNode] = srcMap
			continue
		}
		dst, in := dsts[ls.toNode]
		if !in {
			link := Link{fromNode: ls.fromNode, toNode: ls.toNode}
			link.states = append(link.states, *ls)

			dsts[ls.toNode] = link
			continue
		}

		dst.states = append(dst.states, *ls)
		dsts[ls.toNode] = dst
	}

	return n, nil
}

// Query enables to Controller to determine the current link-state at a time quantum.
func (n *NetworkTypology) Query(msg QueryMsg) bool {
	links, in := n.links[msg.FromNode]
	if !in {
		return false
	}

	link, in := links[msg.ToNode]
	if !in {
		return false
	}

	return link.isUp(msg.AtTime)
}
