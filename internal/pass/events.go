package pass

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type runDir struct {
	dir   string
	wrote bool
}

func openRun(dir string) (*runDir, error) {
	if dir == "" {
		return nil, exitCode(1, "OVERTO_RUN is not set.")
	}
	for _, sub := range []string{"event", "cmd", "stage"} {
		if st, err := os.Stat(filepath.Join(dir, sub)); err != nil || !st.IsDir() {
			return nil, exitCode(1, "the transfer stopped. Nothing was written.")
		}
	}
	return &runDir{dir: dir}, nil
}

func (r *runDir) event(name, body string) error {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	dir := filepath.Join(r.dir, "event")
	tmp := filepath.Join(dir, ".part-"+name)
	if err := os.WriteFile(tmp, []byte(body), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

func (r *runDir) finish(err error) error {
	if r.wrote {
		return nil
	}
	r.wrote = true
	if err == nil {
		return r.event("result", "done")
	}
	var ee *exitErr
	if errors.As(err, &ee) {
		if ee.msg == "declined" {
			return r.event("result", "declined")
		}
		return r.event("result", fmt.Sprintf("error %d %s", ee.code, ee.msg))
	}
	return r.event("result", "error 1 the transfer stopped. Nothing was written.")
}

func (r *runDir) readID() (plate, password, display string, err error) {
	b, err := os.ReadFile(filepath.Join(r.dir, "id"))
	if err != nil {
		return "", "", "", exitCode(1, "that id is not valid.")
	}
	display = strings.TrimSpace(string(b))
	plate, password, err = parseID(display)
	return plate, password, display, err
}

func (r *runDir) waitCmd(name string, d time.Duration) (string, error) {
	path := filepath.Join(r.dir, "cmd", name)
	dead := time.Now().Add(d)
	for {
		b, err := os.ReadFile(path)
		if err == nil {
			s := strings.TrimSpace(string(b))
			if s == "yes" || s == "no" {
				return s, nil
			}
		}
		if time.Now().After(dead) {
			return "", errNoAnswer()
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writePriv(path string, body []byte) error {
	tmp := path + ".part"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
