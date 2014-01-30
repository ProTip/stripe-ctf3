// level3 project main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"index/suffixarray"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type QueryResult struct {
	Results []string `json:"results"`
	Success bool     `json:"success"`
}

type StatusCheck struct {
	Success bool `json:"success"`
}

type WordEntry struct {
	indexEntries []WordIndexEntry
	substrRange  []int
}

type WordIndexEntry struct {
	pathIndex  int
	lineNumber int
}

var wordIndex map[string]WordEntry

var dictionary [][]byte
var workingDirectory string
var pathToIndex map[string]int
var IndexToPath []string
var isIndex bool

func main() {
	runtime.GOMAXPROCS(2)
	IndexToPath = make([]string, 0, 0)
	pathToIndex = make(map[string]int)
	wordIndex = make(map[string]WordEntry)
	isIndex = false
	var port string
	if len(os.Args) > 1 {
		if os.Args[1] != "--master" {
			port = ":909" + os.Args[2]
		} else {
			port = ":9090"
		}
	} else {
		fmt.Println("Changing dir.")
		port = ":9090"
		go doIndexPath("/home/gzapp/projects/stripe-cft3/level3/test/data/input/")
	}
	_ = port
	http.HandleFunc("/index", http_index)
	http.HandleFunc("/", http_query)
	http.HandleFunc("/healthcheck", http_healthcheck)
	http.HandleFunc("/isIndexed", http_is_indexed)
	http.ListenAndServe(port, nil)
}

func http_healthcheck(resp http.ResponseWriter, req *http.Request) {
	resp.Header()["ProTip"] = []string{"Approved"}
	status := StatusCheck{true}
	b, err := json.Marshal(status)
	if err != nil {
		panic(err.Error())
	}
	resp.Write(b)
}

func http_is_indexed(resp http.ResponseWriter, req *http.Request) {
	resp.Header()["ProTip"] = []string{"Approved"}
	status := StatusCheck{isIndex}
	b, err := json.Marshal(status)
	if err != nil {
		panic(err.Error())
	}
	resp.Write(b)
}

func http_index(resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Query().Get("path")
	fmt.Println("Asked to index path: ", path)
	go doIndexPath(path)
	resp.Header()["ProTip"] = []string{"Approved"}
	resp.Write([]byte("Hello from ProTip!"))
}

func http_query(resp http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("q")
	//fmt.Println("Received query for: ", query)
	result := QueryResult{doQuery(query), true}
	b, err := json.Marshal(result)
	if err != nil {
		panic(err.Error())
	}

	//b := doFakeQuery(query)
	resp.Header()["ProTip"] = []string{"Approved"}
	resp.Write(b)
}

func doFakeQuery(query string) []byte {
	var result QueryResult
	grep := exec.Command("grep", "-n", "-R", query)
	out, err := grep.Output()
	if err != nil {
		result = QueryResult{[]string{""}, false}
	} else {
		result_string := strings.Split(string(out), "\n")
		result_string = result_string[:len(result_string)-1]
		for i := range result_string {
			split_string := strings.Split(result_string[i], ":")
			split_string = split_string[:2]
			result_string[i] = strings.Join(split_string, ":")
		}
		result = QueryResult{result_string, true}
	}

	b, err := json.Marshal(result)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func doQuery(word string) []string {
	entry := wordIndex[word]
	entries := wordIndex[word].indexEntries
	if len(entry.substrRange) > 0 {
		subEntries := entriesForRange(entry.substrRange)
		entries = append(entries, subEntries...)
	}
	results := make([]string, 0, 0)

	found := make(map[string]bool)
	for _, entry := range entries {
		result := IndexToPath[entry.pathIndex] + ":" + strconv.Itoa(entry.lineNumber)
		if !found[result] {
			found[result] = true
			results = append(results, result)
		}
	}
	return results
}

func entriesForRange(r []int) []WordIndexEntry {
	//fmt.Println("Getting substrings")
	entries := make([]WordIndexEntry, 0, 0)
	for _, val := range r {
		word := string(dictionary[val])
		entries = append(entries, wordIndex[word].indexEntries...)
	}
	return entries
}

func doIndexPath(path string) {
	workingDirectory = path
	loadWords()
	os.Chdir(path)
	buildPathMap(path)
	indexPaths()
	isIndex = true
	//printPathMaps()
}

func printWordIndex() {
	fmt.Println(wordIndex)
}

func loadWords() {
	fmt.Println("Building word list")
	wordBytes, err := ioutil.ReadFile("words")
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Creating suffix array")
	index := suffixarray.New(wordBytes)
	fmt.Println("Suffix array created")
	_ = index

	wordBytes = wordBytes[:len(wordBytes)-1]
	wordSlice := bytes.Split(wordBytes, []byte("\n"))
	dictionary = wordSlice
	var tempMap = make(map[string]int)
	fmt.Println("Creating tempory dictionary map")
	for i, word := range wordSlice {

		tempMap[string(word)] = i
	}

	for _, val := range wordSlice {
		b := make([]int, 0, 0)
		//for x := 0; x < len(wordSlice)-1; x++ {
		//	if !strings.Contains(string(wordSlice[i]), string(val)) && i != x {
		//		b = append(b, x)
		//	}
		//}
		match := regexp.MustCompile(string(val))
		result := index.FindAllIndex(match, -1)
		if len(result) > 1 {
			for _, offsets := range result {
				var start int
				var end int
				for x := offsets[0]; x > offsets[0]-26 && x > -1; x-- {
					if wordBytes[x] == byte('\n') {
						start = x + 1
						break
					}
				}
				for x := offsets[1]; x < offsets[1]+26 && x < len(wordBytes); x++ {
					if wordBytes[x] == byte('\n') || x == len(wordBytes) {
						end = x
						break
					} else if wordBytes[x] == ' ' {
						break
					}
				}
				if end == 0 {
					end = len(wordBytes)
				}

				substr := string(wordBytes[start:end])

				if substr != string(val) {
					substrIndex := tempMap[substr]
					b = append(b, substrIndex)
					if string(val) == "topsail" {
						fmt.Println("Extracted substrings [", substr, "] ?")
						fmt.Println("At index ", substrIndex, " : ", string(dictionary[substrIndex]))
					}

				}
			}
		}

		_ = result
		wordIndexEntry := make([]WordIndexEntry, 0, 0)
		wordEntry := WordEntry{wordIndexEntry, b}
		wordIndex[string(val)] = wordEntry
	}
}

func buildPathMap(path string) {
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(workingDirectory, path)
		if err != nil {
			panic(err.Error())
		}
		IndexToPath = append(IndexToPath, relPath)
		pathToIndex[relPath] = len(IndexToPath) - 1
		return nil
	})
	if err != nil {
		panic(err.Error())
	}
}

func indexPaths() {
	wordMatch := regexp.MustCompile(`\w+`)
	_ = wordMatch
	for i, val := range IndexToPath {
		_ = i
		fmt.Print(".")
		runtime.GC()
		docBytes, err := ioutil.ReadFile(val)
		if err != nil {
			panic(err.Error())
		}
		docBytes = docBytes[:len(docBytes)-1]
		docLines := bytes.Split(docBytes, []byte("\n"))
		for l, line := range docLines {
			words := wordMatch.FindAll(line, -1)
			for _, word := range words {
				if _, ok := wordIndex[string(word)]; ok {
					entry := wordIndex[string(word)]
					lineNumber := l + 1
					pathIndex := pathToIndex[val]
					entry.indexEntries = append(entry.indexEntries, WordIndexEntry{pathIndex, lineNumber})
					wordIndex[string(word)] = entry
				}
			}
		}
	}
}

func printPathMaps() {
	for i := range IndexToPath {
		fmt.Println("Index ", i, " in IndexToPath points to ", IndexToPath[i])
		fmt.Println("Path ", IndexToPath[i], " in PathToIndex points to ", pathToIndex[IndexToPath[i]])
	}
}
