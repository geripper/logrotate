package logrotate

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/geripper/logrotate/internal/ticker"
)

type RotateLog struct {
	file *os.File

	filename           string
	logPath            string
	stdout             bool
	stderr             bool
	maxAge             time.Duration
	deleteFileWildcard string

	mutex  *sync.Mutex
	rotate <-chan time.Time // notify rotate event
	close  chan struct{}    // close file and write goroutine
}

func NewRoteteLog(logPath string, opts ...Option) (*RotateLog, error) {
	rl := &RotateLog{
		mutex:   &sync.Mutex{},
		close:   make(chan struct{}, 1),
		logPath: logPath,
	}
	for _, opt := range opts {
		opt(rl)
	}

	if err := os.Mkdir(filepath.Dir(rl.logPath), 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}
	rl.filename = path.Base(rl.logPath)

	if err := rl.rotateFile(time.Now()); err != nil {
		return nil, err
	}

	go rl.handleEvent()

	return rl, nil
}

func (r *RotateLog) Write(b []byte) (int, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	n, err := r.file.Write(b)
	return n, err
}

func (r *RotateLog) Close() error {
	r.close <- struct{}{}
	return r.file.Close()
}

func (r *RotateLog) handleEvent() {
	for {
		select {
		case <-r.close:
			return
		case now := <-r.rotate:
			r.rotateFile(now)
		}
	}
}

func (r *RotateLog) rotateFile(now time.Time) error {
	nextRotateTime := ticker.CalRotateTimeDuration(now)
	r.rotate = time.After(nextRotateTime)

	latestLogPath := r.getLatestLogPath(now)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	file, err := os.OpenFile(latestLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	if r.file != nil {
		r.file.Close()
	}
	r.file = file

	if r.stdout {
		os.Stdout = r.file
	}
	if r.stderr {
		os.Stderr = r.file
	}

	r.logPath = latestLogPath

	if r.maxAge > 0 && len(r.deleteFileWildcard) > 0 { // at present
		go r.deleteExpiredFile(now)
	}

	return nil
}

// Judege expired by laste modify time
func (r *RotateLog) deleteExpiredFile(now time.Time) {
	if r.maxAge <= 0 {
		return
	}

	cutoffTime := now.Add(-r.maxAge)

	filePath := filepath.Dir(r.logPath)
	walkFunc := func(path string, info os.FileInfo, err error) error {
		// 如果遍历到的是文件，则输出文件名
		if !info.IsDir() {
			if r.maxAge > 0 && info.ModTime().After(cutoffTime) {
				return nil
			}

			if info.Name() == filepath.Base(r.logPath) {
				return nil
			}

			os.Remove(path)
		}
		return nil
	}

	// 遍历文件夹及其子文件夹，并执行回调函数
	filepath.Walk(filePath, walkFunc)
}

func (r *RotateLog) getLatestLogPath(t time.Time) string {
	filesuffix := path.Ext(r.filename)
	fileprefix := r.filename[0 : len(r.filename)-len(filesuffix)]
	return fmt.Sprintf("%s%s%s-%s%s", filepath.Dir(r.logPath), string([]byte{filepath.Separator}), fileprefix, t.Format("2006-01-02"), filesuffix)
}
