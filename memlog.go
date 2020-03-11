package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
)

type lineBuffer struct {
	buffer         []byte
	size, linesize int
	trunc          string
	start, end     int
	length         int
	closed         bool
	sync.Mutex
}

type lineBufferReader struct {
	lb       *lineBuffer
	size     int
	position int
}

type memlogentry struct {
	tag string
	idx int
}

type memlog struct {
	entry memlogentry
	log   *lineBuffer
}

func newlineBuffer(buffsize, linesize int, truncstr string) *lineBuffer {
	if !strings.HasSuffix(truncstr, "\n") {
		truncstr += "\n"
	}
	if linesize > buffsize {
		linesize = buffsize
	}
	l := &lineBuffer{
		buffer:   make([]byte, buffsize),
		size:     buffsize,
		linesize: linesize,
		trunc:    truncstr,
		length:   0,
		closed:   false,
	}
	return l
}

func (m *lineBuffer) close() {
	m.Lock()
	defer m.Unlock()
	m.closed = true
}

// writeLine writes a line to the memlog, up to m.linesize length
func (m *lineBuffer) writeLine(line string) {
	m.Lock()
	defer m.Unlock()
	if m.closed {
		return
	}
	if !strings.HasSuffix(line, "\n") {
		line = line + "\n"
	}
	var lbytes []byte
	if len(line) > m.linesize {
		lbytes = make([]byte, m.linesize)
		copy(lbytes, []byte(line)[0:(m.linesize-len(m.trunc))])
		copy(lbytes[m.linesize-len(m.trunc):m.linesize], []byte(m.trunc))
	} else {
		lbytes = []byte(line)
	}
	lsize := len(lbytes)
	// Copy string and move m.end
	copied := copy(m.buffer[m.end:], lbytes)
	if copied != lsize {
		copy(m.buffer, lbytes[copied:])
	}
	fallbackStart := m.end
	if fallbackStart == m.size {
		fallbackStart = 0
	}
	m.end += lsize
	if m.end > m.size {
		m.end -= m.size
	}
	if lsize == m.size {
		m.length = m.size
		return
	}
	m.length += lsize
	if m.length > m.size {
		// overlap - end passed start, need to move start and shorten
		if m.end == m.size {
			m.start = 0
		} else {
			m.start = m.end
		}
		m.length = m.size
		// Now scan for the next newline and move start to there
		limit := m.size - lsize
		inc := bytes.IndexByte(m.buffer[m.start:], byte('\n'))
		if inc == -1 {
			inc = len(m.buffer[m.start:])
			inc += bytes.IndexByte(m.buffer, byte('\n'))
		}
		// move start past the "\n"
		inc++
		if inc >= limit {
			// use fallback
			m.start = fallbackStart
			m.length = lsize
			return
		}
		m.length -= inc
		m.start += inc
		if m.start >= m.size {
			m.start -= m.size
		}
	}
}

// getReader returns a memreader from a memlog
func (m *lineBuffer) getReader() (lineBufferReader, error) {
	m.Lock()
	defer m.Unlock()
	if !m.closed {
		return lineBufferReader{}, errors.New("Not closed")
	}
	mr := lineBufferReader{
		lb:       m,
		position: 0,
		size:     m.length,
	}
	return mr, nil
}

// copyReader locks the linebuffer and returns a reader for
// a copy.
func (m *lineBuffer) copyReader() lineBufferReader {
	m.Lock()
	defer m.Unlock()
	lb := &lineBuffer{
		buffer: make([]byte, m.size),
		size:   m.size,
		start:  m.start,
		end:    m.end,
		length: m.length,
		closed: true,
		Mutex:  sync.Mutex{},
	}
	copy(lb.buffer, m.buffer)
	mr := lineBufferReader{
		lb:       lb,
		position: 0,
		size:     m.length,
	}
	return mr
}

// Read implements Read() for a memlog
func (mr lineBufferReader) Read(p []byte) (int, error) {
	rsize := len(p)
	eof := false
	if mr.position+rsize > mr.size {
		eof = true
		rsize = mr.size - mr.position
	}
	m := mr.lb
	rpos := m.start + mr.position
	if rpos > m.size {
		rpos -= m.size
	}
	if rpos+rsize <= m.size {
		copy(p, m.buffer[rpos:rpos+rsize])
	} else {
		copy(p, m.buffer[rpos:m.size])
		copy(p[len(m.buffer[rpos:m.size]):], m.buffer[0:rsize-len(m.buffer[rpos:m.size])])
	}
	mr.position += rsize
	if eof {
		return rsize, io.EOF
	}
	return rsize, nil
}

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
}
