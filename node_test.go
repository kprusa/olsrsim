package main

import (
	"reflect"
	"testing"
)

func Test_updateOneHopNeighbors(t *testing.T) {
	type args struct {
		msg             *HelloMessage
		oneHopNeighbors map[NodeID]OneHopNeighborEntry
		time            int
		holdTime        int
		id              NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]OneHopNeighborEntry
	}{
		{
			name: "new unidirectional neighbor",
			args: args{
				msg: &HelloMessage{
					src:    1,
					unidir: nil,
					bidir:  []NodeID{2, 3},
					mpr:    nil,
				},
				oneHopNeighbors: map[NodeID]OneHopNeighborEntry{
					NodeID(2): {
						neighborID: 1,
						state:      Unidirectional,
						holdUntil:  15,
					},
				},
				time:     10,
				holdTime: 10,
				id:       0,
			},
			want: map[NodeID]OneHopNeighborEntry{
				NodeID(2): {
					neighborID: 1,
					state:      Unidirectional,
					holdUntil:  15,
				},
				NodeID(1): {
					neighborID: 1,
					state:      Unidirectional,
					holdUntil:  20,
				},
			},
		},
		{
			name: "new bidirectional neighbor",
			args: args{
				msg: &HelloMessage{
					src:    1,
					unidir: nil,
					bidir:  []NodeID{0, 2, 3},
					mpr:    nil,
				},
				oneHopNeighbors: map[NodeID]OneHopNeighborEntry{
					NodeID(1): {
						neighborID: 1,
						state:      Unidirectional,
						holdUntil:  15,
					},
					NodeID(2): {
						neighborID: 1,
						state:      Unidirectional,
						holdUntil:  15,
					},
				},
				time:     10,
				holdTime: 10,
				id:       0,
			},
			want: map[NodeID]OneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      Bidirectional,
					holdUntil:  20,
				},
				NodeID(2): {
					neighborID: 1,
					state:      Unidirectional,
					holdUntil:  15,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateOneHopNeighbors(tt.args.msg, tt.args.oneHopNeighbors, tt.args.time, tt.args.holdTime, tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateOneHopNeighbors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateTwoHopNeighbors(t *testing.T) {
	type args struct {
		msg             *HelloMessage
		twoHopNeighbors map[NodeID]map[NodeID]NodeID
		id              NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]map[NodeID]NodeID
	}{
		// TODO: Add test cases.
		{
			name: "new two hop",
			args: args{
				msg: &HelloMessage{
					src:    1,
					unidir: nil,
					bidir:  []NodeID{2},
					mpr:    nil,
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{},
				id:              0,
			},
			want: map[NodeID]map[NodeID]NodeID{
				NodeID(1): {
					NodeID(2): NodeID(2),
				},
			},
		},
		{
			name: "delete previous entries",
			args: args{
				msg: &HelloMessage{
					src:    1,
					unidir: nil,
					bidir:  []NodeID{3},
					mpr:    nil,
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{
					NodeID(1): {
						NodeID(2): NodeID(2),
					},
				},
				id: 0,
			},
			want: map[NodeID]map[NodeID]NodeID{
				NodeID(1): {
					NodeID(3): NodeID(3),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateTwoHopNeighbors(tt.args.msg, tt.args.twoHopNeighbors, tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateTwoHopNeighbors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_calculateMPRs(t *testing.T) {
	type args struct {
		oneHopNeighbors map[NodeID]OneHopNeighborEntry
		twoHopNeighbors map[NodeID]map[NodeID]NodeID
	}
	tests := []struct {
		name string
		args args
		want map[NodeID]OneHopNeighborEntry
	}{
		{
			name: "ensure greedy",
			args: struct {
				oneHopNeighbors map[NodeID]OneHopNeighborEntry
				twoHopNeighbors map[NodeID]map[NodeID]NodeID
			}{
				oneHopNeighbors: map[NodeID]OneHopNeighborEntry{
					NodeID(1): OneHopNeighborEntry{
						neighborID: 1,
						state:      Bidirectional,
						holdUntil:  20,
					},
					NodeID(2): OneHopNeighborEntry{
						neighborID: 1,
						state:      Bidirectional,
						holdUntil:  20,
					},
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{
					NodeID(1): {
						NodeID(3): NodeID(3),
						NodeID(4): NodeID(4),
					},
					NodeID(2): {
						NodeID(3): NodeID(3),
					},
				},
			},
			want: map[NodeID]OneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      MPR,
					holdUntil:  20,
				},
				NodeID(2): OneHopNeighborEntry{
					neighborID: 1,
					state:      Bidirectional,
					holdUntil:  20,
				},
			},
		},
		{
			name: "ensure coverage",
			args: struct {
				oneHopNeighbors map[NodeID]OneHopNeighborEntry
				twoHopNeighbors map[NodeID]map[NodeID]NodeID
			}{
				oneHopNeighbors: map[NodeID]OneHopNeighborEntry{
					NodeID(1): OneHopNeighborEntry{
						neighborID: 1,
						state:      Bidirectional,
						holdUntil:  20,
					},
					NodeID(2): OneHopNeighborEntry{
						neighborID: 1,
						state:      Bidirectional,
						holdUntil:  20,
					},
				},
				twoHopNeighbors: map[NodeID]map[NodeID]NodeID{
					NodeID(1): {
						NodeID(3): NodeID(3),
					},
					NodeID(2): {
						NodeID(4): NodeID(4),
					},
				},
			},
			want: map[NodeID]OneHopNeighborEntry{
				NodeID(1): {
					neighborID: 1,
					state:      MPR,
					holdUntil:  20,
				},
				NodeID(2): OneHopNeighborEntry{
					neighborID: 1,
					state:      MPR,
					holdUntil:  20,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateMPRs(tt.args.oneHopNeighbors, tt.args.twoHopNeighbors); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calculateMPRs() = %v, want %v", got, tt.want)
			}
		})
	}
}