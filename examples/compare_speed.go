package main

import (
	"log"
	"math/rand"
	"runtime"
	"time"

	"github.com/cbergoon/merkletree"
	merkletree_threading "github.com/tyrm/merkletree-threading"
	"golang.org/x/crypto/blake2b"
)

//TestContent implements the Content interface provided by merkletree and represents the content stored in the tree.
type TestContent struct {
	x string
}

type TestContent2 struct {
	x string
}

//CalculateHash hashes the values of a TestContent
func (t TestContent) CalculateHash() ([]byte, error) {
	h, err := blake2b.New512(nil)
	if err != nil {
		return nil, err
	}

	if _, err := h.Write([]byte(t.x)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

//CalculateHash hashes the values of a TestContent
func (t TestContent2) CalculateHash() ([]byte, error) {
	h, err := blake2b.New512(nil)
	if err != nil {
		return nil, err
	}

	if _, err := h.Write([]byte(t.x)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}


//Equals tests for equality of two Contents
func (t TestContent) Equals(other merkletree.Content) (bool, error) {
	return t.x == other.(TestContent).x, nil
}

//Equals tests for equality of two Contents
func (t TestContent2) Equals(other merkletree_threading.Content) (bool, error) {
	return t.x == other.(TestContent2).x, nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringBytesMaskImpr(n int) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func main() {
	//Build list of Content to build tree
	log.Println("Building test tree")
	start := time.Now()
	var list []merkletree.Content
	var list2 []merkletree_threading.Content
	for i := 1; i <= 10000; i++ {
		testItem := RandStringBytesMaskImpr(10000)
		list = append(list, TestContent{x: testItem})
		list2 = append(list2, TestContent2{x: testItem})

	}
	elapsed := time.Since(start)
	log.Printf("Building test tree took %s", elapsed)

	//Create a new Merkle Tree from the list of Content
	log.Println("Creating merkletree")
	start = time.Now()
	t, err := merkletree.NewTree(list)
	if err != nil {
		log.Fatal(err)
	}
	elapsed = time.Since(start)
	log.Printf("Creating merkletree took %s", elapsed)

	//Get the Merkle Root of the tree
	mr := t.MerkleRoot()
	log.Println(mr)

	//Create a new Merkle Tree from the list of Content
	log.Println("Creating merkletree_threading")
	start = time.Now()
	t2, err := merkletree_threading.NewTree(list2, runtime.NumCPU())
	if err != nil {
		log.Fatal(err)
	}
	elapsed = time.Since(start)
	log.Printf("Creating merkletree_threading took %s", elapsed)

	//Get the Merkle Root of the tree
	mr2 := t2.MerkleRoot()
	log.Println(mr2)

	//Verify the entire tree (hashes for each node) is valid
	log.Println("Verifying merkletree")
	start = time.Now()
	vt, err := t.VerifyTree()
	if err != nil {
		log.Fatal(err)
	}
	elapsed = time.Since(start)
	log.Printf("Verifying merkle1tree took %s", elapsed)
	log.Println("Verify Tree: ", vt)

	//Verify the entire tree (hashes for each node) is valid
	log.Println("Verifying merkletree_threading")
	start = time.Now()
	vt2, err := t2.VerifyTree()
	if err != nil {
		log.Fatal(err)
	}
	elapsed = time.Since(start)
	log.Printf("Verifying merkletree_threading took %s", elapsed)
	log.Println("Verify Tree: ", vt2)

	//Verify a specific content in in the tree
	//vc, err := t.VerifyContent(list[0])
	//if err != nil {
		//log.Fatal(err)
	//}

	//log.Println("Verify Content: ", vc)

	//String representation
	log.Printf("%v", t.Leafs[0].Parent.Parent)
	log.Printf("%v", t2.Leafs[0].Parent.Parent)
}