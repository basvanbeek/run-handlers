// Copyright (c) Bas van Beek 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package filewatcher provides a file watcher service for use with the run package.
package filewatcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/basvanbeek/multierror"
	"github.com/basvanbeek/run"
	"github.com/basvanbeek/run/pkg/flag"
	"github.com/basvanbeek/telemetry/scope"
	"github.com/fsnotify/fsnotify"
)

var log = scope.Register("file-watcher", "file watcher service")

type fileReg struct {
	name            string
	defaultFilePath string
	ch              chan []byte
}

type Service struct {
	mtx sync.RWMutex
	f   []*fileReg
	w   *fsnotify.Watcher
	p   map[string]int

	initialized int32
}

func (s *Service) Name() string {
	return "file-watcher"
}

func (s *Service) AddWatcher(name, fqn string) (<-chan []byte, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, reg := range s.f {
		if strings.EqualFold(reg.name, name) {
			return nil, errors.New("registration already exists")
		}
	}

	if atomic.LoadInt32(&s.initialized) == 1 {
		// we are already running the watcher...

		// get the path in which our file is located
		fp := filepath.Dir(fqn)

		if _, err := os.Stat(fp); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("path %s does not exist: %w", fp, err)
			}
			if os.IsPermission(err) {
				return nil, fmt.Errorf("path %s permission denied: %w", fp, err)
			}
			return nil, fmt.Errorf("failed to check path %s: %w", fp, err)
		}

		s.p[fp]++
		ch := make(chan []byte)
		s.f = append(s.f, &fileReg{
			name:            name,
			defaultFilePath: fqn,
			ch:              ch,
		})
		if s.p[fp] < 2 {
			// new patch to watch
			if err := s.w.Add(fp); err != nil {
				// remove the registration
				s.f = s.f[:len(s.f)-1]
				close(ch)
				return nil, fmt.Errorf("failed to add file watcher for %s: %w",
					name, err)
			}
		}
		return ch, nil
	}

	ch := make(chan []byte)
	s.f = append(s.f, &fileReg{
		name:            name,
		defaultFilePath: fqn,
		ch:              ch,
	})

	return ch, nil
}

func (s *Service) RemoveWatcher(name string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for i, reg := range s.f {
		if strings.EqualFold(reg.name, name) {
			s.f = append(s.f[:i], s.f[i+1:]...)

			if atomic.LoadInt32(&s.initialized) == 1 {
				// we are already running the watcher...

				fp := filepath.Dir(reg.defaultFilePath)
				s.p[fp]--
				if s.p[fp] < 1 {
					// no more watchers for this path, we can remove it
					delete(s.p, fp)
					if err := s.w.Remove(fp); err != nil {
						return fmt.Errorf(
							"failed to remove file watcher for %s: %w",
							name, err)
					}
				}
			}
			close(reg.ch)
			return nil
		}
	}

	return fmt.Errorf("registration %s not found", name)
}

func (s *Service) FlagSet() *run.FlagSet {
	flags := run.NewFlagSet("File watcher options")

	for _, reg := range s.f {
		flags.StringVar(&reg.defaultFilePath, "fwatch-"+reg.name,
			reg.defaultFilePath, "Watch file patch for "+reg.name)
	}

	return flags
}

func (s *Service) Validate() error {
	var mErr error

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	for _, reg := range s.f {
		if reg.defaultFilePath == "" {
			mErr = multierror.Append(mErr,
				flag.NewValidationError("fwatch-"+reg.name,
					errors.New("file path cannot be empty")),
			)
		}
	}

	return mErr
}

func (s *Service) PreRun() (err error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	if s.p == nil {
		s.p = make(map[string]int)
	}
	s.w, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	for _, reg := range s.f {
		fp := filepath.Dir(reg.defaultFilePath)
		if _, err = os.Stat(fp); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("path %s does not exist: %w", fp, err)
			}
			return fmt.Errorf("failed to check path %s: %w", fp, err)
		}
		s.p[fp]++
		if s.p[fp] < 2 {
			// new patch to watch
			if err = s.w.Add(fp); err != nil {
				return fmt.Errorf("failed to add file watcher for %s: %w",
					reg.name, err)
			}
		}
	}

	// we are now initialized
	atomic.StoreInt32(&s.initialized, 1)

	return nil
}

func (s *Service) ServeContext(ctx context.Context) (err error) {
forLoop:
	for {
		select {
		case <-ctx.Done():
			// exit the loop
			break forLoop
		case event, ok := <-s.w.Events:
			if !ok {
				log.Error("file watcher event channel closed unexpectedly", err)
				err = fmt.Errorf("file watcher event channel closed unexpectedly: %w", err)
				break forLoop
			}
			log.Debug("file watcher event",
				"name", event.Name, "op", event.Op)
			if event.Op&fsnotify.Write != fsnotify.Write && event.Op&fsnotify.Create != fsnotify.Create {
				// file not modified or created
				continue
			}

			var (
				onKubernetes bool
				kubeDir      string
			)
			// test if event.Name ends with /..data
			// this signals we are on a kubernetes cluster
			// we specifically look for the <volumemount>/..data directory
			// which is the default for kubernetes symlink wizardry.
			// a create event on it tells us that the secret has been modified
			if event.Op.Has(fsnotify.Create) && strings.HasSuffix(event.Name, "/..data") {
				kubeDir = filepath.Dir(event.Name)
				onKubernetes = true
			}

			s.mtx.RLock()
			for _, reg := range s.f {
				if onKubernetes {
					// kubernetes filter
					if !strings.EqualFold(kubeDir, filepath.Dir(reg.defaultFilePath)) {
						continue
					}
					event.Name = reg.defaultFilePath
				} else {
					// local file filter
					if !strings.EqualFold(event.Name, reg.defaultFilePath) {
						continue
					}
				}

				log.Debug("file watcher event",
					"name", reg.name, "event", event.Name,
					"op", event.Op)
				// try to load the file
				var b []byte
				b, err = os.ReadFile(event.Name)
				if err != nil {
					log.Error("failed to read file", err,
						"name", reg.name, "event", event.Name, "op", event.Op)
					continue
				}
				reg.ch <- b
			}
			s.mtx.RUnlock()
		case err2, ok := <-s.w.Errors:
			if !ok {
				log.Error("file watcher error channel closed unexpectedly", err2)
				err = fmt.Errorf("file watcher error channel closed unexpectedly: %w", err2)
				break forLoop
			}
			log.Error("received a file watcher error", err2)
		}
	}

	s.mtx.Lock()
	for _, reg := range s.f {
		close(reg.ch)
	}
	s.mtx.Unlock()
	err2 := s.w.Close()
	if err == nil {
		err = err2
	}
	return
}

var (
	_ run.Config         = (*Service)(nil)
	_ run.PreRunner      = (*Service)(nil)
	_ run.ServiceContext = (*Service)(nil)
)
