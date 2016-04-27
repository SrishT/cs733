package fs

import (
	_ "fmt"
	"sync"
	"time"
)

type FileInfo struct {
	filename   string
	contents   []byte
	version    int
	absexptime time.Time
	timer      *time.Timer
}

type FS struct {
	sync.RWMutex
	dir map[string]*FileInfo
}

type FileServer struct {
	fs 	 *FS
	gversion int //global version	
}

func Initialize()(*FileServer) {
	var fs = &FS{dir: make(map[string]*FileInfo, 1000)}
	f := &FileServer{fs,0}
	return f
}

func (fi *FileInfo) cancelTimer() {
	if fi.timer != nil {
		fi.timer.Stop()
		fi.timer = nil
	}
}

func (f *FileServer) ProcessMsg(msg *Msg) *Msg {
	switch msg.Kind {
	case 'r':
		return f.processRead(msg)
	case 'w':
		return f.processWrite(msg)
	case 'c':
		return f.processCas(msg)
	case 'd':
		return f.processDelete(msg)
	}

	// Default: Internal error. Shouldn't come here since
	// the msg should have been validated earlier.
	return &Msg{Kind: 'I'}
}

func (f *FileServer) processRead(msg *Msg) *Msg {
	f.fs.RLock()
	defer f.fs.RUnlock()
	if fi := f.fs.dir[msg.Filename]; fi != nil {
		remainingTime := 0
		if fi.timer != nil {
			remainingTime := int(fi.absexptime.Sub(time.Now()))
			if remainingTime < 0 {
				remainingTime = 0
			}
		}
		return &Msg{
			Kind:     'C',
			Filename: fi.filename,
			Contents: fi.contents,
			Numbytes: len(fi.contents),
			Exptime:  remainingTime,
			Version:  fi.version,
		}
	} else {
		return &Msg{Kind: 'F'} // file not found
	}
}

func (f *FileServer) internalWrite(msg *Msg) *Msg {
	fi := f.fs.dir[msg.Filename]
	if fi != nil {
		fi.cancelTimer()
	} else {
		fi = &FileInfo{}
	}

	f.gversion += 1
	fi.filename = msg.Filename
	fi.contents = msg.Contents
	fi.version = f.gversion

	var absexptime time.Time
	if msg.Exptime > 0 {
		dur := time.Duration(msg.Exptime) * time.Second
		absexptime = time.Now().Add(dur)
		timerFunc := func(name string, ver int) func() {
			return func() {
				f.processDelete(&Msg{Kind: 'D',
					Filename: name,
					Version:  ver})
			}
		}(msg.Filename, f.gversion)

		fi.timer = time.AfterFunc(dur, timerFunc)
	}
	fi.absexptime = absexptime
	f.fs.dir[msg.Filename] = fi

	return ok(f.gversion)
}

func (f *FileServer) processWrite(msg *Msg) *Msg {
	f.fs.Lock()
	defer f.fs.Unlock()
	return f.internalWrite(msg)
}

func (f *FileServer) processCas(msg *Msg) *Msg {
	f.fs.Lock()
	defer f.fs.Unlock()

	if fi := f.fs.dir[msg.Filename]; fi != nil {
		if msg.Version != fi.version {
			return &Msg{Kind: 'V', Version: fi.version}
		}
	}
	return f.internalWrite(msg)
}

func (f *FileServer) processDelete(msg *Msg) *Msg {
	//--debug
	f.fs.Lock()
	defer f.fs.Unlock()
	fi := f.fs.dir[msg.Filename]
	if fi != nil {
		if msg.Version > 0 && fi.version != msg.Version {
			// non-zero msg.Version indicates a delete due to an expired timer
			return nil // nothing to do
		}
		fi.cancelTimer()
		delete(f.fs.dir, msg.Filename)
		return ok(0)
	} else {
		return &Msg{Kind: 'F'} // file not found
	}

}

func ok(version int) *Msg {
	return &Msg{Kind: 'O', Version: version}
}
