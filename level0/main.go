// level0 project main.go
package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"math"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"
)

var dic map[string]bool
var file_contents_split [][]byte
var working_dir string
var files_contents []byte
var byte_dic []byte
var lookup_table []uint32
var stdin []byte

func main() {
	runtime.GOMAXPROCS(2)
	log.SetOutput(os.Stderr)
	load_lookup_unsafe()
	copy_words()
	load_stdin()
	parse()
}

func copy_words() {
	defer un(trace("Copying word list"))
	byte_dic = words
}

func parse() {
	defer un(trace("Parse"))
	split := find_split()
	c1 := make(chan []byte)
	c2 := make(chan []byte)
	go byte_parse_input_byte(stdin[0:split], c1)
	go byte_parse_input_byte(stdin[split:], c2)
	out1, out2 := <-c1, <-c2
	os.Stdout.Write(out1)
	os.Stdout.Write(out2)
}

func find_split() int {
	split := int(math.Floor(float64(len(stdin) / 2)))
	for i := 0; i < 24; i++ {
		if stdin[split+i] == byte('\n') || stdin[split+i] == byte(' ') {
			split += i
			break
		}
	}
	return split
}

func load_words() {
	defer un(trace("Loading words"))
	var err error
	byte_dic, err = ioutil.ReadFile("words_pruned")
	if err != nil {
		panic(err.Error())
	}
}

func load_lookup_unsafe() {
	defer un(trace("Load lookup unsafe"))
	//raw, err := ioutil.ReadFile("lookup")
	//if err != nil {
	//	panic(err.Error())
	//}

	header := *(*reflect.SliceHeader)(unsafe.Pointer(&raw_lookup))
	header.Len /= 4
	header.Cap /= 4

	lookup_table = *(*[]uint32)(unsafe.Pointer(&header))
	//fmt.Println(len(lookup_table))
}

func load_stdin() {
	var err error
	stdin, err = ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err.Error())
	}
}

func byte_parse_input_byte(input []byte, c chan []byte) {
	defer un(trace("Byte parse input"))
	var buffer bytes.Buffer
	var slice = input
	checkpoint := 0
	for i := 0; i < len(slice); i++ {

		if slice[i] == byte('.') {
			buffer.Write(encase_word(slice[checkpoint : i+1]))
			checkpoint = i + 1
		} else if slice[i] == byte(' ') || slice[i] == byte('\n') {
			if checkpoint == i {
				buffer.WriteByte(slice[i])
				checkpoint = i + 1
			} else {
				buffer.Write(byte_replace_byte(slice[checkpoint:i]))
				if slice[i] == byte(' ') || slice[i] == byte('\n') {
					buffer.WriteByte(slice[i])
				}
				checkpoint = i + 1
			}
		} else if i+1 == len(slice) {
			buffer.Write(byte_replace_byte(slice[checkpoint : i+1]))
		}
	}
	c <- buffer.Bytes()
}

//func needle_lookup(needle string) bool {
//	byte_needle := []byte(needle)
//	i := sort.Search(len(file_contents_split), func(i int) bool {
//		return bytes.Compare(file_contents_split[i], byte_needle) >= 0
//	})
//	if i < len(file_contents_split) && bytes.Equal(file_contents_split[i], byte_needle) {
//		return true
//	} else {
//		return false
//	}
//}

//func byte_needle_lookup(byte_needle []byte) bool {
//	i := sort.Search(len(file_contents_split), func(i int) bool {
//		return bytes.Compare(file_contents_split[i], byte_needle) >= 0
//	})
//	if i < len(file_contents_split) && bytes.Equal(file_contents_split[i], byte_needle) {
//		return true
//	} else {
//		return false
//	}
//}

func lookup_needle_lookup(byte_needle []byte) bool {
	i := sort.Search(int(len(lookup_table)), func(i int) bool {
		return bytes.Compare(slice_for_needle(i, byte_needle), byte_needle) >= 0
	})
	//if i < len(lookup_table) && bytes.Equal(file_contents_split[i], byte_needle) {
	//	return true
	if i < len(lookup_table) && bytes.Equal(slice_for_needle(i, byte_needle), byte_needle) {
		return true
	} else {
		return false
	}
}

func slice_for_needle(index int, needle []byte) []byte {
	seek := int(lookup_table[index])
	scan := seek + len(needle)
	if scan >= len(byte_dic) {
		scan = len(byte_dic) - 1
	}

	if byte_dic[scan] != byte('\n') {
		scan += 1
	}
	dic_slice := byte_dic[seek:scan]
	//fmt.Println("Comparing <", needle, "> to <", dic_slice, ">")
	return dic_slice
}

func replace(in string) string {
	in_lower := strings.ToLower(in)
	if dic[in_lower] == true {
		return in
	} else {
		return "<" + in + ">"
	}
}

//func byte_replace(in string) string {
//	if needle_lookup(strings.ToLower(in)) {
//		return in
//	} else {
//		return "<" + in + ">"
//	}
//}

func byte_replace_byte(in []byte) []byte {
	if lookup_needle_lookup(bytes.ToLower(in)) {
		return in
	} else {
		return encase_word(in)
	}
}

func encase_word(in []byte) []byte {
	var buffer bytes.Buffer
	buffer.WriteString("<")
	buffer.Write(in)
	buffer.WriteString(">")
	return buffer.Bytes()
}

func trace(s string) (string, time.Time) {
	log.Println("START:", s)
	return s, time.Now()
}

func un(s string, startTime time.Time) {
	endTime := time.Now()
	log.Println("  END:", s, "ElapsedTime in seconds:", endTime.Sub(startTime))
}
