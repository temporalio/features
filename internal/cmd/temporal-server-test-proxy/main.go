package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrUnknownCommand = errors.New("unknown command")
	ErrAlreadyStarted = errors.New("already started")
	ErrAlreadyStopped = errors.New("already stopped")
	ErrAlreadyFrozen  = errors.New("already frozen")
	ErrAlreadyThawed  = errors.New("already thawed")
)

var (
	flagTrace  bool
	flagListen string
	flagDial   string

	gLastID uint32

	gFrozenMutex sync.Mutex
	gFrozenCond  *sync.Cond
	gFrozen      bool

	gListenerWG sync.WaitGroup
	gListener   net.Listener

	gPairWG       sync.WaitGroup
	gPairMapMutex sync.Mutex
	gPairMap      map[uint32]*Pair
)

func init() {
	flag.BoolVar(&flagTrace, "trace", false, "enable tracing logs")
	flag.StringVar(&flagListen, "listen", "", "TCP host:port to listen on")
	flag.StringVar(&flagDial, "dial", "", "TCP host:port to connect to")
}

func init() {
	gFrozenCond = sync.NewCond(&gFrozenMutex)
}

func main() {
	flag.Parse()

	if flagListen == "" {
		Fatal(1, "must specify -listen")
		panic(nil)
	}
	if flagDial == "" {
		Fatal(1, "must specify -dial")
		panic(nil)
	}

	if err := ActionStart(); err != nil {
		Fatal(2, "failed to listen on %q: %v", flagListen, err)
		panic(nil)
	}
	defer IgnoreResult(ActionStop)

	fmt.Println("ready")
	scanner := bufio.NewScanner(os.Stdin)
	looping := true
	for looping && scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")

		var err error
		switch line {
		case "":
			fallthrough
		case "noop":
			err = nil

		case "exit":
			fallthrough
		case "quit":
			err = io.EOF

		case "start":
			err = ActionStart()

		case "stop":
			err = ActionStop()

		case "kill-all":
			err = ActionKillAll()

		case "freeze":
			err = ActionFreeze()

		case "thaw":
			err = ActionThaw()

		default:
			err = ErrUnknownCommand
		}

		switch {
		case err == nil:
			fmt.Println("ok")
		case err == io.EOF:
			fmt.Println("bye")
			looping = false
		default:
			fmt.Printf("err %q\n", err.Error())
		}
	}
	err := scanner.Err()
	if IsErrClosed(err) {
		err = nil
	}
	if err != nil {
		Fatal(2, "I/O error reading from stdin: %v", err)
		panic(nil)
	}
}

func ActionStart() error {
	if gListener != nil {
		return ErrAlreadyStarted
	}

	l, err := net.Listen("tcp", flagListen)
	if err != nil {
		return err
	}

	gListener = l
	gListenerWG.Add(1)
	go AcceptLoop(l)
	return nil
}

func ActionStop() error {
	if gListener == nil {
		return ErrAlreadyStopped
	}

	err := gListener.Close()
	gListener = nil
	gListenerWG.Wait()
	if IsErrClosed(err) {
		err = nil
	}
	return err
}

func ActionKillAll() error {
	var pairMap map[uint32]*Pair
	gPairMapMutex.Lock()
	pairMap, gPairMap = gPairMap, nil
	gPairMapMutex.Unlock()

	for _, pair := range pairMap {
		pair.Close()
	}
	gPairWG.Wait()
	return nil
}

func ActionFreeze() error {
	gFrozenMutex.Lock()
	defer gFrozenMutex.Unlock()

	if gFrozen {
		return ErrAlreadyFrozen
	}
	gFrozen = true
	return nil
}

func ActionThaw() error {
	gFrozenMutex.Lock()
	defer gFrozenMutex.Unlock()

	if !gFrozen {
		return ErrAlreadyThawed
	}
	gFrozen = false
	gFrozenCond.Broadcast()
	return nil
}

func AcceptLoop(l net.Listener) {
	defer gListenerWG.Done()
	defer func() {
		err := l.Close()
		if IsErrClosed(err) {
			err = nil
		}
		if err != nil {
			Warn("error while closing listener: %v", err)
		}
	}()

	for {
		AwaitThaw()

		Trace("accepting")
		c, err := l.Accept()
		if IsErrClosed(err) {
			return
		}
		if err != nil {
			Error("accept failed")
			return
		}

		id := atomic.AddUint32(&gLastID, 1)
		Trace("accept complete [#%d]", id)

		pair := &Pair{ID: id, In: c}
		gPairMapMutex.Lock()
		if gPairMap == nil {
			gPairMap = make(map[uint32]*Pair, 4)
		}
		gPairMap[id] = pair
		gPairMapMutex.Unlock()

		gPairWG.Add(1)
		go pair.Thread(c)
	}
}

func AwaitThaw() {
	gFrozenMutex.Lock()
	for gFrozen {
		gFrozenCond.Wait()
	}
	gFrozenMutex.Unlock()
}

type Pair struct {
	ID    uint32
	Mutex sync.Mutex
	In    net.Conn
	Out   net.Conn
}

func (pair *Pair) Thread(in net.Conn) {
	defer gPairWG.Done()
	defer pair.Close()

	inLA := in.LocalAddr().String()
	inRA := in.RemoteAddr().String()
	badge := fmt.Sprintf("#%d; %s->%s", pair.ID, inRA, inLA)

	Trace("dialing [%s]", badge)
	out, err := net.Dial("tcp", flagDial)
	if err != nil {
		Error("failed to dial %q: %v [%s]", flagDial, err, badge)
		return
	}

	outLA := out.LocalAddr().String()
	outRA := out.RemoteAddr().String()
	badge = badge + "; " + outLA + "->" + outRA
	Trace("dial complete [%s]", badge)

	if !pair.Activate(out) {
		IgnoreResult(out.Close)
		return
	}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	gPairWG.Add(2)
	go pair.CopyLoop(ch1, in, out, badge+"; server->client")
	go pair.CopyLoop(ch2, out, in, badge+"; client->server")
	select {
	case <-ch1:
	case <-ch2:
	}
}

func (pair *Pair) Activate(out net.Conn) bool {
	pair.Mutex.Lock()
	defer pair.Mutex.Unlock()

	if pair.In == nil {
		return false
	}
	pair.Out = out
	return true
}

func (pair *Pair) CopyLoop(ch chan struct{}, dst net.Conn, src net.Conn, badge string) {
	defer gPairWG.Done()
	defer close(ch)
	for {
		const BufferSize = 1 << 12 // 4 KiB
		var buffer [BufferSize]byte

		AwaitThaw()

		n, err := src.Read(buffer[:])
		if IsErrClosed(err) {
			return
		}
		if err != nil {
			Error("read I/O error: %v [%s]", err, badge)
			return
		}

		if n <= 0 {
			continue
		}

		AwaitThaw()

		_, err = dst.Write(buffer[:n])
		if IsErrClosed(err) {
			return
		}
		if err != nil {
			Error("write I/O error: %v [%s]", err, badge)
			return
		}
	}
}

func (pair *Pair) Close() {
	gPairMapMutex.Lock()
	delete(gPairMap, pair.ID)
	gPairMapMutex.Unlock()

	var (
		in  net.Conn
		out net.Conn
	)
	pair.Mutex.Lock()
	in, pair.In = pair.In, nil
	out, pair.Out = pair.Out, nil
	pair.Mutex.Unlock()

	if in == nil {
		return
	}

	inLA := in.LocalAddr().String()
	inRA := in.RemoteAddr().String()
	badge := fmt.Sprintf("#%d; %s->%s", pair.ID, inRA, inLA)
	inErr := in.Close()

	var outErr error
	if out != nil {
		outLA := out.LocalAddr().String()
		outRA := out.RemoteAddr().String()
		badge = badge + "; " + outLA + "->" + outRA
		outErr = out.Close()
	}

	if IsErrClosed(inErr) {
		inErr = nil
	}
	if IsErrClosed(outErr) {
		outErr = nil
	}
	if inErr != nil {
		Warn("error while closing incoming connection: %v [%s]", inErr, badge)
	}
	if outErr != nil {
		Warn("error while closing outgoing connection: %v [%s]", outErr, badge)
	}
	Trace("connection closed [%s]", badge)
}

func IgnoreResult(fn func() error) {
	_ = fn()
}

func IsErrClosed(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, io.EOF):
		return true
	case errors.Is(err, fs.ErrClosed):
		return true
	case errors.Is(err, net.ErrClosed):
		return true
	default:
		return false
	}
}

func Trace(format string, args ...any) {
	if !flagTrace {
		return
	}
	fmt.Fprintf(os.Stderr, "trace: "+format+"\n", args...)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warn: "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

func Fatal(rc int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", args...)
	os.Exit(rc)
}
