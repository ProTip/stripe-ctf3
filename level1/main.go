// level1 project main.go
package main

import (
	"crypto/sha1"
	//"fmt"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	//"reflect"
	"hash"
	"strconv"
	"time"
	//"unsafe"
	"math/rand"
	"runtime"
	"sync"
)

type miner_message struct {
	mined          bool
	commit_message []byte
}

var wg sync.WaitGroup

func main() {
	os.Chdir("/home/gzapp/projects/stripe-cft3/level1/level1")
	max_procs := 4
	max_miners := 4
	runtime.GOMAXPROCS(max_procs)
	log.SetOutput(os.Stderr)
	git_tree := git_write_tree()
	git_parent := git_parnet()

	time := timestamp()
	difficulty := get_difficulty()

	//fmt.Println("Git tree is: ", string(git_tree))
	//fmt.Println("Git parent is: ", string(git_parent))
	//fmt.Println("Difficulty is: ", string(difficulty))
	//fmt.Println("Timestamp is: ", string(time))

	c1 := make(chan miner_message, max_miners)

	for i := 0; i < max_miners; i++ {
		commit_message := construct_commit(git_tree, git_parent, time)
		go find_coin(commit_message, difficulty, c1)
		wg.Add(1)
	}

	for i := 0; i < max_miners; i++ {
		resp := <-c1
		if resp.mined == true {
			fmt.Println(string(resp.commit_message))
			wg.Wait()
			os.Exit(0)
		}
	}
	wg.Wait()
	os.Exit(1)
}

func find_coin(commit_message []byte, difficulty []byte, c chan miner_message) {
	defer go_un(trace("Find coin."))
	rng_source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(rng_source)
	local_diff := make([]byte, len(difficulty), len(difficulty))
	copy(local_diff, difficulty)
	start_nounce := uint64(rng.Int63())
	end_nounce := start_nounce + 40000000
	log.Println("Staring at nounce: ", start_nounce)
	log.Println("Ending at nounce: ", end_nounce)

	nounce_slice := make([]byte, 16, 16)
	nounce_buffer := make([]byte, 8, 8)
	hasher := sha1.New()

	commit_object := construct_commit_hash(commit_message)
	nounce_slice = commit_object[len(commit_object)-16:]

	for i := start_nounce; i < end_nounce; i++ {
		nounce_message(commit_object, nounce_slice, nounce_buffer, uint64(i))
		//fmt.Println(string(commit_message))
		sha1_message(hasher, commit_object)
		//fmt.Println(string(commit_message))
		if bytes.Compare(hasher.Sum(nil), local_diff) == -1 {
			nounce_slice = commit_message[len(commit_message)-16:]
			nounce_message(commit_message, nounce_slice, nounce_buffer, uint64(i))
			resp := miner_message{true, commit_message}
			c <- resp
			return
		}
	}
	resp := miner_message{false, nil}
	c <- resp
}

func compare(sum []byte, diff []byte) int {
	//fmt.Println("Comparing sum: ", sum, " to difficulty: ", diff)
	return bytes.Compare(sum, diff)
}

func git_commit_hash(message []byte) []byte {
	commit := construct_commit_hash(message)
	commit_reader := bytes.NewReader(commit)
	h := sha1.New()
	io.Copy(h, commit_reader)
	sum := h.Sum(nil)
	return sum[:]
}

func sha1_message(hasher hash.Hash, message []byte) {
	commit_reader := bytes.NewReader(message)
	io.Copy(hasher, commit_reader)
}

func construct_commit_hash(message []byte) []byte {
	var commit bytes.Buffer

	length := len(message)
	commit.Write([]byte("commit "))
	commit.WriteString(strconv.Itoa(length + 1))
	commit.WriteByte(byte(0x00))
	commit.Write(message)
	commit.WriteByte(byte('\n'))
	return commit.Bytes()
}

func nounce_message(message []byte, nounce_slice []byte, nounce_buffer []byte, nounce uint64) {
	binary.BigEndian.PutUint64(nounce_buffer, nounce)
	hex.Encode(nounce_slice, nounce_buffer)
}

func construct_commit(tree []byte, parent []byte, time []byte) []byte {
	var message bytes.Buffer
	message.Write([]byte("tree "))
	message.Write(tree[:len(tree)-1])
	message.Write([]byte("\n"))

	message.Write([]byte("parent "))
	message.Write(parent[:len(parent)-1])
	message.Write([]byte("\n"))

	message.Write([]byte("author ProTip user <greg.zapp@gmail.com> "))
	message.Write(time[:len(time)-1])
	message.Write([]byte("+0000\n"))
	message.Write([]byte("committer ProTip user <greg.zapp@gmail.com> "))
	message.Write(time[:len(time)-1])
	message.Write([]byte("+0000\n\n"))

	message.Write([]byte("Give me a Gitcoin\n\n"))
	message.Write([]byte("0000000000000000"))
	return message.Bytes()
}

func timestamp() []byte {
	cmd := exec.Command("date", "+%s")
	out, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}
	return out
}

func get_difficulty() []byte {
	cmd := exec.Command("cat", "difficulty.txt")
	out, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}

	zeros := []byte("0000000000000000000000000000000000000000")
	difficulty := out[:len(out)-1]
	difficulty_slice := zeros[:len(out)-1]
	copy(difficulty_slice, difficulty)
	var diff_binary = make([]byte, 40, 40)
	hex.Decode(diff_binary, zeros)

	return diff_binary
}

func git_parnet() []byte {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}
	return out
}

func git_write_tree() []byte {
	cmd := exec.Command("git", "write-tree")
	out, err := cmd.Output()
	if err != nil {
		panic(err.Error())
	}
	return out
}

func trace(s string) (string, time.Time) {
	log.Println("START:", s)
	return s, time.Now()
}

func un(s string, startTime time.Time) {
	endTime := time.Now()
	log.Println("  END:", s, "ElapsedTime in seconds:", endTime.Sub(startTime))
}

func go_un(s string, startTime time.Time) {
	un(s, startTime)
	wg.Done()
}
