// Copyright 2017 Cameron Bergoon
// Licensed under the MIT License, see LICENCE file for details.

package merkletree

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"sync"
)

//Content represents the data that is stored and verified by the tree. A type that
//implements this interface can be used as an item in the tree.
type Content interface {
	CalculateHash() ([]byte, error)
	Equals(other Content) (bool, error)
}

//LeafPile is a thread compatible holder for collecting leafs
type LeafPile struct {
	leafs map[int]*Node
	lock  sync.RWMutex
}

//MerkleTree is the container for the tree. It holds a pointer to the root of the tree,
//a list of pointers to the leaf nodes, and the merkle root.
type MerkleTree struct {
	Root       *Node
	merkleRoot []byte
	Leafs      []*Node
}

//Node represents a node, root, or leaf in the tree. It stores pointers to its immediate
//relationships, a hash, the content stored if it is a leaf, and other metadata.
type Node struct {
	Parent *Node
	Left   *Node
	Right  *Node
	leaf   bool
	dup    bool
	Hash   []byte
	C      Content
}

//verifyNode walks down the tree until hitting a leaf, calculating the hash at each level
//and returning the resulting hash of Node n.
func (n *Node) verifyNode() ([]byte, error) {
	if n.leaf {
		return n.C.CalculateHash()
	}

	leftBytes  := make(chan []byte)
	leftNode   := make(chan *Node)
	rightBytes := make(chan []byte)
	rightNode  := make(chan *Node)

	go verifyNodeWorker(leftNode, leftBytes)
	go verifyNodeWorker(rightNode, rightBytes)

	leftNode <- n.Left
	rightNode <- n.Right

	leftByteOutput := <-leftBytes
	rightByteOutput := <-rightBytes

	h := sha256.New()
	if _, err := h.Write(append(leftByteOutput, rightByteOutput...)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func verifyNodeWorker(in chan *Node, out chan []byte) {
	node := <- in
	bytes, err := node.verifyNode()
	if err != nil {
		//TODO: fix error return
		log.Printf("Error verifying node %s", err)
	}
	out <- bytes
}

//calculateNodeHash is a helper function that calculates the hash of the node.
func (n *Node) calculateNodeHash() ([]byte, error) {
	if n.leaf {
		return n.C.CalculateHash()
	}

	h := sha256.New()
	if _, err := h.Write(append(n.Left.Hash, n.Right.Hash...)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

//NewTree creates a new Merkle Tree using the content cs.
func NewTree(cs []Content, workers int) (*MerkleTree, error) {
	root, leafs, err := buildWithContent(cs, workers)
	if err != nil {
		return nil, err
	}
	t := &MerkleTree{
		Root:       root,
		merkleRoot: root.Hash,
		Leafs:      leafs,
	}
	return t, nil
}

//buildWithContent is a helper function that for a given set of Contents, generates a
//corresponding tree and returns the root node, a list of leaf nodes, and a possible error.
//Returns an error if cs contains no Contents.
func buildWithContent(cs []Content, workers int) (*Node, []*Node, error) {
	if len(cs) == 0 {
		return nil, nil, errors.New("error: cannot construct tree with no content")
	}

	// Calculate Hashes
	var leafpile LeafPile
	leafpile.leafs = make(map[int]*Node)

	ch := make(chan map[int]Content)
	var wg sync.WaitGroup

	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go buildWithContentWorker(&wg, i, ch, &leafpile)
	}

	for order, c := range cs {
		aLeaf := make(map[int]Content)
		aLeaf[order] = c
		ch <- aLeaf
	}
	close(ch)
	wg.Wait()

	// Sort
	var leafs []*Node
	for i := 0; i < len(leafpile.leafs); i++ {
		leafs = append(leafs, leafpile.leafs[i])
	}

	// Add Dupe if Needed
	if len(leafs)%2 == 1 {
		duplicate := &Node{
			Hash: leafs[len(leafs)-1].Hash,
			C:    leafs[len(leafs)-1].C,
			leaf: true,
			dup:  true,
		}
		leafs = append(leafs, duplicate)
	}
	root, err := buildIntermediate(leafs)
	if err != nil {
		return nil, nil, err
	}

	return root, leafs, nil
}

//buildWithContentWorker is a worker function for buildWithContent
func buildWithContentWorker(wg *sync.WaitGroup, id int, leafs chan map[int]Content, pile *LeafPile) {
	defer wg.Done()
	for c := range leafs {
		for order, leaf := range c {
			hash, err := leaf.CalculateHash()
			if err != nil {
				//TODO: fix error return
				log.Printf("Error calculating hash %s", err)
			}

			pile.lock.Lock()
			pile.leafs[order] =  &Node{
				Hash: hash,
				C:    leaf,
				leaf: true,
			}
			pile.lock.Unlock()
		}
	}
}

//buildIntermediate is a helper function that for a given list of leaf nodes, constructs
//the intermediate and root levels of the tree. Returns the resulting root node of the tree.
func buildIntermediate(nl []*Node) (*Node, error) {
	var nodes []*Node
	for i := 0; i < len(nl); i += 2 {
		h := sha256.New()
		var left, right int = i, i + 1
		if i+1 == len(nl) {
			right = i
		}
		chash := append(nl[left].Hash, nl[right].Hash...)
		if _, err := h.Write(chash); err != nil {
			return nil, err
		}
		n := &Node{
			Left:  nl[left],
			Right: nl[right],
			Hash:  h.Sum(nil),
		}
		nodes = append(nodes, n)
		nl[left].Parent = n
		nl[right].Parent = n
		if len(nl) == 2 {
			return n, nil
		}
	}
	return buildIntermediate(nodes)
}

//MerkleRoot returns the unverified Merkle Root (hash of the root node) of the tree.
func (m *MerkleTree) MerkleRoot() []byte {
	return m.merkleRoot
}

//RebuildTree is a helper function that will rebuild the tree reusing only the content that
//it holds in the leaves.
func (m *MerkleTree) RebuildTree(workers int) error {
	var cs []Content
	for _, c := range m.Leafs {
		cs = append(cs, c.C)
	}
	root, leafs, err := buildWithContent(cs, workers)
	if err != nil {
		return err
	}
	m.Root = root
	m.Leafs = leafs
	m.merkleRoot = root.Hash
	return nil
}

//RebuildTreeWith replaces the content of the tree and does a complete rebuild; while the root of
//the tree will be replaced the MerkleTree completely survives this operation. Returns an error if the
//list of content cs contains no entries.
func (m *MerkleTree) RebuildTreeWith(cs []Content, workers int) error {
	root, leafs, err := buildWithContent(cs, workers)
	if err != nil {
		return err
	}
	m.Root = root
	m.Leafs = leafs
	m.merkleRoot = root.Hash
	return nil
}

//VerifyTree verify tree validates the hashes at each level of the tree and returns true if the
//resulting hash at the root of the tree matches the resulting root hash; returns false otherwise.
func (m *MerkleTree) VerifyTree() (bool, error) {
	calculatedMerkleRoot, err := m.Root.verifyNode()
	if err != nil {
		return false, err
	}

	if bytes.Compare(m.merkleRoot, calculatedMerkleRoot) == 0 {
		return true, nil
	}
	return false, nil
}

//VerifyContent indicates whether a given content is in the tree and the hashes are valid for that content.
//Returns true if the expected Merkle Root is equivalent to the Merkle root calculated on the critical path
//for a given content. Returns true if valid and false otherwise.
func (m *MerkleTree) VerifyContent(content Content) (bool, error) {
	for _, l := range m.Leafs {
		ok, err := l.C.Equals(content)
		if err != nil {
			return false, err
		}

		if ok {
			currentParent := l.Parent
			for currentParent != nil {
				h := sha256.New()
				rightBytes, err := currentParent.Right.calculateNodeHash()
				if err != nil {
					return false, err
				}

				leftBytes, err := currentParent.Left.calculateNodeHash()
				if err != nil {
					return false, err
				}
				if currentParent.Left.leaf && currentParent.Right.leaf {
					if _, err := h.Write(append(leftBytes, rightBytes...)); err != nil {
						return false, err
					}
					if bytes.Compare(h.Sum(nil), currentParent.Hash) != 0 {
						return false, nil
					}
					currentParent = currentParent.Parent
				} else {
					if _, err := h.Write(append(leftBytes, rightBytes...)); err != nil {
						return false, err
					}
					if bytes.Compare(h.Sum(nil), currentParent.Hash) != 0 {
						return false, nil
					}
					currentParent = currentParent.Parent
				}
			}
			return true, nil
		}
	}
	return false, nil
}

//String returns a string representation of the tree. Only leaf nodes are included
//in the output.
func (m *MerkleTree) String() string {
	s := ""
	for _, l := range m.Leafs {
		s += fmt.Sprint(l)
		s += "\n"
	}
	return s
}
