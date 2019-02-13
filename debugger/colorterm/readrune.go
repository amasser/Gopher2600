package colorterm

/* this part of the colorterm package facilitates the reading of runes from a
* reader interface (eg, os.Stdin). a straight call to reader.ReadRune() will
* block until input is available. this causes problems if we want the program
* to wait for something else from another reader or channel. with the mechanism
* here, an input loop can use a channel-select to prevent the blocking.
 */

import (
	"bufio"
	"os"
)

type readRune struct {
	r   rune
	n   int
	err error
}

type runeReader chan readRune

func initRuneReader() runeReader {
	reader := bufio.NewReader(os.Stdin)
	ch := make(runeReader)
	go func() {
		var readRune readRune
		for {
			readRune.r, readRune.n, readRune.err = reader.ReadRune()
			ch <- readRune
		}
	}()

	return ch
}