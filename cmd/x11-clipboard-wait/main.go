package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xfixes"
	"github.com/BurntSushi/xgb/xproto"
)

var prog = filepath.Base(os.Args[0])

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", prog)
	fmt.Fprintf(os.Stderr, "  %s\n", prog)
	fmt.Fprintf(os.Stderr, "(the command takes no options)\n")
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(prog + ": ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	X, err := xgb.NewConn()
	if err != nil {
		return fmt.Errorf("opening x11 connection: %v", err)
	}
	defer X.Close()

	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)

	primary := xproto.Atom(xproto.AtomPrimary)

	const atomName = "CLIPBOARD"
	atomReply, err := xproto.InternAtom(X, false, uint16(len(atomName)), atomName).Reply()
	if err != nil {
		return fmt.Errorf("no %s atom: %v", atomName, err)
	}
	clip := atomReply.Atom

	// https://www.x.org/releases/X11R7.7/doc/fixesproto/fixesproto.txt
	if err := xfixes.Init(X); err != nil {
		return fmt.Errorf("xfixes init: %v", err)
	}
	const (
		xfixesMajorVersion = 5
		xfixesMinorVersion = 0
	)
	xfixesVersion, err := xfixes.QueryVersion(X, xfixesMajorVersion, xfixesMinorVersion).Reply()
	if err != nil {
		return fmt.Errorf("xfixes version handshake: %v", err)
	}
	if (xfixesVersion.MajorVersion < xfixesMajorVersion) ||
		(xfixesVersion.MajorVersion == xfixesMajorVersion && xfixesVersion.MinorVersion < xfixesMinorVersion) {
		return fmt.Errorf("x11 xfixes extension	is too old: %d.%d", xfixesVersion.MajorVersion, xfixesVersion.MinorVersion)
	}

	_ = xfixes.SelectSelectionInput(X, screen.Root, primary, xfixes.SelectionEventMaskSetSelectionOwner)
	_ = xfixes.SelectSelectionInput(X, screen.Root, clip, xfixes.SelectionEventMaskSetSelectionOwner).Check()

loop:
	for {
		ev, err := X.WaitForEvent()
		if err != nil {
			return fmt.Errorf("waiting for selection: %v", err)
		}
		if ev == nil {
			return io.EOF
		}

		switch ev := ev.(type) {
		default:
			fmt.Printf("unknown event: %#v", ev)
		case xfixes.SelectionNotifyEvent:
			switch ev.Subtype {
			default:
				return fmt.Errorf("unexpected xfixes selection event type: %d", ev.Subtype)
			case xfixes.SelectionEventSetSelectionOwner:
				break loop
			}
		}
	}
	return nil
}
