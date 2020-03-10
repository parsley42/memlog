package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
)

const logSize = 1024
const maxMemLine = 64
const lineTrunc = "<... truncated>\n"

type memlogentry struct {
	tag string
	idx int
}

type memlog struct {
	entry      memlogentry
	log        []byte
	start, end int
	closed     bool
	sync.Mutex
}

type memreader struct {
	log      *memlog
	size     int
	position int
}

// writeLine writes a line to the memlog, up to maxMemLine length
func (m *memlog) writeLine(line string) {
	m.Lock()
	defer m.Unlock()
	if m.closed {
		return
	}
	if !strings.HasSuffix(line, "\n") {
		line = line + "\n"
	}
	var lbytes []byte
	if len(line) > maxMemLine {
		lbytes = make([]byte, maxMemLine)
		copy(lbytes, []byte(line)[0:(maxMemLine-len(lineTrunc))])
		copy(lbytes[maxMemLine-len(lineTrunc):maxMemLine], []byte(lineTrunc))
	} else {
		lbytes = []byte(line)
	}
	lsize := len(lbytes)
	fmt.Printf("DEBUG: line is '%s', start: %d, end: %d, len: %d\n", strings.TrimRight(string(lbytes), "\n"), m.start, m.end, lsize)
	if m.end+lsize > logSize { // wrap
		tailSize := m.end + lsize - logSize
		copy(m.log[m.end:], lbytes[0:(tailSize)])
		headSize := lsize - tailSize
		copy(m.log[0:], lbytes[headSize:])
		m.end = m.end + lsize - logSize
		fmt.Printf("DEBUG new end is: %d\n", m.end)
		offset := bytes.IndexByte(m.log[m.end+1:logSize], byte('\n'))
		fmt.Printf("DEBUG offiset is: %d\n", offset)
		m.start = m.end + 1 + offset + 1
		fmt.Printf("DEBUG new start is: %d\n", m.start)
	} else {
		copy(m.log[m.end:], lbytes)
		if m.end >= m.start {
			m.end += len(lbytes)
		} else {
			if m.end+len(lbytes) > m.start {
				offset := bytes.IndexByte(m.log[m.end+len(lbytes):], byte('\n'))
				m.start += offset
			}
			m.end += len(lbytes)
		}
	}
}

// getReader returns a memreader from a memlog
func (m *memlog) getReader() memreader {
	mr := memreader{
		log:      m,
		position: 0,
	}
	if m.end >= m.start {
		mr.size = m.end - m.start
	} else {
		mr.size = logSize - (m.start - m.end)
	}
	return mr
}

// Read implements Read() for a memlog
func (mr memreader) Read(p []byte) (int, error) {
	rsize := len(p)
	eof := false
	if mr.position+rsize > mr.size {
		eof = true
		rsize = mr.size - mr.position
	}
	m := mr.log
	rpos := m.start + mr.position
	if rpos > logSize {
		rpos -= logSize
	}
	if rpos+rsize <= logSize {
		copy(p, m.log[rpos:rpos+rsize])
	} else {
		copy(p, m.log[rpos:logSize])
		copy(p[len(m.log[rpos:logSize]):], m.log[0:rsize-len(m.log[rpos:logSize])])
	}
	mr.position += rsize
	if eof {
		return rsize, io.EOF
	}
	return rsize, nil
}

func main() {
	ml := &memlog{
		log:    make([]byte, logSize),
		start:  0,
		end:    0,
		closed: false,
		Mutex:  sync.Mutex{},
	}
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
		pline := strings.TrimRight(line, "\n")
		fmt.Printf("Writing line %d: %s, length: %d\n", i, pline, len(line))
		ml.writeLine(line)
		fmt.Printf("Buffer - start: %d, end: %d\n", ml.start, ml.end)
		if ml.end < ml.start {
			os.Stdout.Write(ml.log[ml.start:logSize])
			os.Stdout.Write([]byte("|"))
			os.Stdout.Write(ml.log[0:ml.end])
		} else {
			os.Stdout.Write(ml.log[ml.start:ml.end])
		}
		fmt.Println()
	}
	fmt.Printf("Finished writing, start: %d, end: %d\n", ml.start, ml.end)
	mlr := ml.getReader()
	rl := bufio.NewScanner(mlr)
	rlen := 0
	for rl.Scan() {
		line := rl.Text()
		rlen += len(line)
		fmt.Println(line)
	}
	fmt.Printf("Read %d bytes\n", rlen)
}
