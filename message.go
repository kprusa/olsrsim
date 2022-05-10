package main

import (
	"fmt"
	"strconv"
	"strings"
)

func separatedString(items []NodeID, sep string) string {
	var strs []string
	for _, item := range items {
		strs = append(strs, strconv.Itoa(int(item)))
	}
	return strings.Join(strs, sep)
}

type MsgType string

const (
	HelloType MsgType = "HELLO"
	DataType  MsgType = "DATA"
	TCType    MsgType = "TC"
)

// HelloMessage represents a HELLO OLSR message.
type HelloMessage struct {
	src    NodeID
	unidir []NodeID
	bidir  []NodeID
	mpr    []NodeID
}

func (m HelloMessage) String() string {
	f := "* %d HELLO UNIDIR %s BIDIR %s MPR %s"
	return fmt.Sprintf(
		f,
		m.src,
		separatedString(m.unidir, " "),
		separatedString(m.bidir, " "),
		separatedString(m.mpr, " "),
	)
}

// DataMessage represents a DATA OLSR message.
type DataMessage struct {
	src     NodeID
	dst     NodeID
	nxtHop  NodeID
	fromnbr NodeID
	data    string
}

func (m DataMessage) String() string {
	f := "%d %d DATA %d %d %s"
	return fmt.Sprintf(f, m.nxtHop, m.fromnbr, m.src, m.dst, m.data)
}

// TCMessage represents a topology control (TC) OLSR message.
type TCMessage struct {
	src     NodeID
	fromnbr NodeID
	seq     uint
	ms      []NodeID
}

func (m TCMessage) String() string {
	f := "* %d TC %d %d MS %s"
	return fmt.Sprintf(f, m.fromnbr, m.src, m.seq, separatedString(m.ms, " "))
}
