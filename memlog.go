package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
)

const maxMailBody = 10485760 // 10MB

func main() {
	ml := newlineBuffer(1024, 64, "<... truncated>")
	for i := 0; i < 64; i++ {
		var line string
		if i%3 == 0 {
			ri := rand.Int()
			line = fmt.Sprintf("This is line %d, with a random integer: %d\n", i, ri)
		} else if i%3 == 1 {
			line = fmt.Sprintf("Now its line %d time for all good men to come to the aid of their country; the quick brown fox jumped over the lazy dog\n", i)
		} else {
			line = fmt.Sprintf("Shortss line %d it doesn't have a newline", i)
		}
		ml.writeLine(line)
		// pline := strings.TrimRight(line, "\n")
		// fmt.Printf("Wrote line %d: %s, length: %d\n", i, pline, len(line))
		// fmt.Printf("Buffer - start: %d, end: %d, length: %d\n", ml.start, ml.end, ml.length)
		// out := make([]byte, ml.length, ml.length+1)
		// copied := copy(out, ml.buffer[ml.start:])
		// if ml.length > copied {
		// 	out[copied] = '|'
		// 	out = append(out, '\n')
		// 	copy(out[copied+1:], ml.buffer)
		// } else {
		// 	out = append(out, '|')
		// }
		// os.Stdout.Write(out)
		// fmt.Println()
	}
	fmt.Printf("Finished writing, start: %d, end: %d, length: %d\n", ml.start, ml.end, ml.length)
	ml.close()
	mlr, err := ml.getReader()
	if err != nil {
		fmt.Printf("Getting reader: %v\n", err)
	}
	rl := bufio.NewScanner(mlr)
	rlen := 0
	for rl.Scan() {
		line := rl.Text()
		rlen += len(line) + 1
		fmt.Println(line)
	}
	fmt.Printf("Read %d bytes\n", rlen)
	fmt.Println("Reading entire buffer with ReadAll...")
	lbr, err := ml.getReader()
	if err != nil {
		fmt.Printf("Getting reader: %v\n", err)
	}
	lr := io.LimitReader(lbr, maxMailBody)
	body := new(bytes.Buffer)
	buff, rerr := ioutil.ReadAll(lr)
	if rerr != nil {
		fmt.Printf("Reading log: %v\n", rerr)
		return
	}
	body.Write(buff)
	fmt.Printf("Reading %d bytes\n", body.Len())
	os.Stdout.Write(body.Bytes())
}
