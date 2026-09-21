package store

import "math/rand"

const (
	skipListMaxLevel = 16
	skipListP        = .5
)

type skipListLevel struct {
	Forward *SkipListNode
	Span    int64
}

type SkipListNode struct {
	Member string
	Score  float64
	Level  []skipListLevel
}

type SkipList struct {
	Head   *SkipListNode
	Length int64
}

func NewSkipList() *SkipList {
	return &SkipList{
		Head: &SkipListNode{Level: make([]skipListLevel, skipListMaxLevel)},
	}
}

func randomLevel() int {
	level := 1
	for rand.Float64() < skipListP && level < skipListMaxLevel {
		level++
	}
	return level
}

func less(score1 float64, member1 string, score2 float64, member2 string) bool {
	if score1 != score2 {
		return score1 < score2
	}
	return member1 < member2
}

func (s *SkipList) Insert(member string, score float64) *SkipListNode {
	update := make([]*SkipListNode, skipListMaxLevel)
	rank := make([]int64, skipListMaxLevel)

	current := s.Head
	for i := skipListMaxLevel - 1; i >= 0; i-- {
		if i == skipListMaxLevel-1 {
			rank[i] = 0
		} else {
			rank[i] = rank[i+1]
		}
		for current.Level[i].Forward != nil &&
			less(current.Level[i].Forward.Score, current.Level[i].Forward.Member, score, member) {
			rank[i] += current.Level[i].Span
			current = current.Level[i].Forward
		}
		update[i] = current
	}

	level := randomLevel()
	node := &SkipListNode{
		Member: member,
		Score:  score,
		Level:  make([]skipListLevel, level),
	}

	for i := 0; i < level; i++ {
		node.Level[i].Forward = update[i].Level[i].Forward
		update[i].Level[i].Forward = node

		node.Level[i].Span = update[i].Level[i].Span - (rank[0] - rank[i])
		update[i].Level[i].Span = (rank[0] - rank[i]) + 1
	}

	for i := level; i < skipListMaxLevel; i++ {
		update[i].Level[i].Span++
	}

	s.Length++
	return node
}

func (s *SkipList) Delete(member string, score float64) bool {
	update := make([]*SkipListNode, skipListMaxLevel)
	current := s.Head

	for i := skipListMaxLevel - 1; i >= 0; i-- {
		for current.Level[i].Forward != nil &&
			less(current.Level[i].Forward.Score, current.Level[i].Forward.Member, score, member) {
			current = current.Level[i].Forward
		}
		update[i] = current
	}

	target := current.Level[0].Forward
	if target == nil || target.Score != score || target.Member != member {
		return false
	}

	for i := 0; i < skipListMaxLevel; i++ {
		if update[i].Level[i].Forward == target {
			update[i].Level[i].Span += target.Level[i].Span - 1
			update[i].Level[i].Forward = target.Level[i].Forward
		} else {
			update[i].Level[i].Span--
		}
	}
	s.Length--
	return true
}

func (s *SkipList) GetRank(member string, score float64) int64 {
	var rank int64
	current := s.Head
	for i := skipListMaxLevel - 1; i >= 0; i-- {
		for current.Level[i].Forward != nil &&
			less(current.Level[i].Forward.Score, current.Level[i].Forward.Member, score, member) {
			rank += current.Level[i].Span
			current = current.Level[i].Forward
		}
	}
	current = current.Level[0].Forward
	if current != nil && current.Score == score && current.Member == member {
		return rank + 1
	}
	return 0
}

func (s *SkipList) GetByRank(rank int64) *SkipListNode {
	if rank <= 0 || rank > s.Length {
		return nil
	}
	var traversed int64
	current := s.Head
	for i := skipListMaxLevel - 1; i >= 0; i-- {
		for current.Level[i].Forward != nil && traversed+current.Level[i].Span <= rank {
			traversed += current.Level[i].Span
			current = current.Level[i].Forward
		}
		if traversed == rank {
			return current
		}
	}
	return nil
}

func (s *SkipList) Range(start, end int64) []*SkipListNode {
	if start < 1 {
		start = 1
	}
	if end > s.Length {
		end = s.Length
	}
	if start > end {
		return nil
	}

	nodes := make([]*SkipListNode, 0, end-start+1)
	for node := s.GetByRank(start); node != nil && start <= end; start++ {
		nodes = append(nodes, node)
		node = node.Level[0].Forward
	}
	return nodes
}
