package main

import (
	"fmt"
	"strconv"
	"strings"

	"9fans.net/go/plan9"
	"9fans.net/go/plan9/client"
)

// StyleLayer represents a single named layer owned by acme-treesitter on the
// acme-styles compositor for one acme window.  No persistent 9P connection is
// held — each operation mounts acme-styles fresh so the layer survives
// acme-styles restarts transparently.
type StyleLayer struct {
	winID   int
	layerID int
}

func mount() (*client.Fsys, error) {
	return client.MountService("acme-styles")
}

// newStyleLayer allocates a fresh layer for winID in the acme-styles
// compositor.  Returns nil (not an error) if acme-styles is not running.
func newStyleLayer(winID int) *StyleLayer {
	sl := &StyleLayer{winID: winID}
	fs, err := mount()
	if err != nil {
		return nil
	}
	defer fs.Close()
	if err := sl.allocLayer(fs); err != nil {
		return nil
	}
	return sl
}

// allocLayer opens <winid>/layers/new and reads back the assigned layer ID.
func (sl *StyleLayer) allocLayer(fs *client.Fsys) error {
	fid, err := fs.Open(fmt.Sprintf("%d/layers/new", sl.winID), plan9.OREAD)
	if err != nil {
		return err
	}
	var buf [32]byte
	n, err := fid.Read(buf[:])
	fid.Close()
	if err != nil && n == 0 {
		return fmt.Errorf("reading layer id: %v", err)
	}
	id, err := strconv.Atoi(strings.TrimSpace(string(buf[:n])))
	if err != nil {
		return fmt.Errorf("parsing layer id %q: %v", string(buf[:n]), err)
	}
	sl.layerID = id
	return nil
}

// Clear zeros the layer; best-effort.  Opening ftClear OWRITE causes
// acme-styles to clear the layer at open; flushLayer fires at fid clunk.
func (sl *StyleLayer) Clear() {
	if sl == nil {
		return
	}
	fs, err := mount()
	if err != nil {
		return
	}
	defer fs.Close()
	fid, err := fs.Open(fmt.Sprintf("%d/layers/%d/clear", sl.winID, sl.layerID), plan9.OWRITE)
	if err != nil {
		return
	}
	fid.Close()
}

// Apply writes entries to the layer.  Opening the style file with OWRITE
// causes acme-styles to clear the layer atomically at open; the combined
// flush (writeCtl -> one winframesync) happens at fid clunk.  This means
// the entire Apply results in exactly ONE ctl write to acme, eliminating
// the intermediate "no style" flash that a separate clear + write caused.
//
// If the layer no longer exists (acme-styles restarted) it is re-allocated
// before writing.
func (sl *StyleLayer) Apply(entries []StyleEntry) error {
	if sl == nil {
		return nil
	}
	fs, err := mount()
	if err != nil {
		return err
	}
	defer fs.Close()

	stylePath := fmt.Sprintf("%d/layers/%d/style", sl.winID, sl.layerID)
	fid, err := fs.Open(stylePath, plan9.OWRITE)
	if err != nil {
		// Layer gone — acme-styles restarted.  Re-allocate and retry.
		if err2 := sl.allocLayer(fs); err2 != nil {
			return fmt.Errorf("open style: %v; re-alloc: %v", err, err2)
		}
		fid, err = fs.Open(fmt.Sprintf("%d/layers/%d/style", sl.winID, sl.layerID), plan9.OWRITE)
		if err != nil {
			return err
		}
	}
	defer fid.Close()

	if len(entries) == 0 {
		return nil // clunk alone flushes an empty composition -> clear
	}
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "%d %d %d\n", e.StyleIdx, e.Start, e.End)
	}
	_, err = fid.Write([]byte(sb.String()))
	return err
}
